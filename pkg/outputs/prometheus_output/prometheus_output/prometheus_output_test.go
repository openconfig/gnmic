package prometheus_output

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"

	"github.com/openconfig/gnmic/pkg/outputs"
)

// initTestOutput creates a prometheusOutput with the given buffer size
// listening on an ephemeral port. Caller must defer Close().
func initTestOutput(t *testing.T, bufferSize int) *prometheusOutput {
	t.Helper()
	cfg := map[string]any{
		"listen":      ":0",
		"path":        "/metrics",
		"expiration":  "60s",
		"timeout":     "10s",
		"num-workers": 4,
		"buffer-size": bufferSize,
	}
	p := &prometheusOutput{}
	err := p.Init(context.Background(), "test-prom", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	return p
}

// makeSubscribeResponse builds a SubscribeResponse with a single int value
// at a unique path so each message produces a distinct metric key.
func makeSubscribeResponse(id int) *gnmi.SubscribeResponse {
	return &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Timestamp: time.Now().UnixNano(),
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{
							Elem: []*gnmi.PathElem{
								{Name: "interface", Key: map[string]string{"name": fmt.Sprintf("eth%d", id)}},
								{Name: "counters"},
								{Name: "in-octets"},
							},
						},
						Val: &gnmi.TypedValue{
							Value: &gnmi.TypedValue_IntVal{IntVal: int64(id)},
						},
					},
				},
			},
		},
	}
}

// collectMetrics runs a Collect and returns all gathered prometheus metrics.
func collectMetrics(p *prometheusOutput) []prometheus.Metric {
	ch := make(chan prometheus.Metric, 100_000)
	done := make(chan struct{})
	var collected []prometheus.Metric
	go func() {
		defer close(done)
		for m := range ch {
			collected = append(collected, m)
		}
	}()
	p.Collect(ch)
	close(ch)
	<-done
	return collected
}

// writeAndUpdate writes totalMsgs through the output, calling Update (with
// newBuf) after the first half have been sent. The second half is written
// concurrently so messages race with the channel swap.
func writeAndUpdate(t *testing.T, p *prometheusOutput, totalMsgs, newBuf int) {
	t.Helper()
	ctx := context.Background()
	meta := outputs.Meta{"subscription-name": "sub1"}

	half := totalMsgs / 2
	for i := 0; i < half; i++ {
		p.Write(ctx, makeSubscribeResponse(i), meta)
	}

	err := p.Update(ctx, map[string]any{
		"listen":      p.cfg.Load().Listen,
		"path":        "/metrics",
		"expiration":  "60s",
		"timeout":     "10s",
		"num-workers": 4,
		"buffer-size": newBuf,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(totalMsgs - half)
	for i := half; i < totalMsgs; i++ {
		go func(id int) {
			defer wg.Done()
			p.Write(ctx, makeSubscribeResponse(id), meta)
		}(i)
	}
	wg.Wait()

	// Let workers process the backlog.
	time.Sleep(500 * time.Millisecond)
}

func TestUpdateBufferShrink(t *testing.T) {
	const (
		initialBuf = 200
		updatedBuf = 50
		totalMsgs  = 500
	)

	p := initTestOutput(t, initialBuf)
	defer p.Close()

	writeAndUpdate(t, p, totalMsgs, updatedBuf)

	collected := collectMetrics(p)
	t.Logf("collected=%d (expected %d)", len(collected), totalMsgs)
	if len(collected) < totalMsgs {
		t.Errorf("messages lost during buffer shrink %d→%d: expected %d, collected %d",
			initialBuf, updatedBuf, totalMsgs, len(collected))
	}
}

func TestUpdateBufferExpand(t *testing.T) {
	const (
		initialBuf = 50
		updatedBuf = 500
		totalMsgs  = 500
	)

	p := initTestOutput(t, initialBuf)
	defer p.Close()

	writeAndUpdate(t, p, totalMsgs, updatedBuf)

	collected := collectMetrics(p)
	t.Logf("collected=%d (expected %d)", len(collected), totalMsgs)
	if len(collected) < totalMsgs {
		t.Errorf("messages lost during buffer expand %d→%d: expected %d, collected %d",
			initialBuf, updatedBuf, totalMsgs, len(collected))
	}
}

func TestUpdateSameBufferSizeNoSwap(t *testing.T) {
	const (
		bufSize   = 200
		totalMsgs = 500
	)

	p := initTestOutput(t, bufSize)
	defer p.Close()

	writeAndUpdate(t, p, totalMsgs, bufSize)

	collected := collectMetrics(p)
	t.Logf("collected=%d (expected %d)", len(collected), totalMsgs)
	if len(collected) != totalMsgs {
		t.Errorf("expected %d metrics, got %d", totalMsgs, len(collected))
	}
}

// freeListenAddr briefly binds to :0 and returns the assigned address.
// The listener is closed before returning so the address can be reused.
func freeListenAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to grab free port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

// initTestOutputWithAddr creates a prometheusOutput bound to addr with the
// given path. Caller must defer Close().
func initTestOutputWithAddr(t *testing.T, addr, path string) *prometheusOutput {
	t.Helper()
	cfg := map[string]any{
		"listen":      addr,
		"path":        path,
		"expiration":  "60s",
		"timeout":     "10s",
		"num-workers": 2,
		"buffer-size": 1000,
	}
	p := &prometheusOutput{}
	err := p.Init(context.Background(), "test-prom", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	return p
}

// scrape performs an HTTP GET and returns (statusCode, body, error).
func scrape(url string) (int, string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body), nil
}

// writeMessages sends n unique messages to the output synchronously.
func writeMessages(p *prometheusOutput, n int) {
	ctx := context.Background()
	meta := outputs.Meta{"subscription-name": "sub1"}
	for i := 0; i < n; i++ {
		p.Write(ctx, makeSubscribeResponse(i), meta)
	}
}

func TestHTTPServerRebuildOnListenChange(t *testing.T) {
	addr1 := freeListenAddr(t)
	addr2 := freeListenAddr(t)
	if addr1 == addr2 {
		t.Skip("got same port twice, skip")
	}

	p := initTestOutputWithAddr(t, addr1, "/metrics")
	defer p.Close()

	writeMessages(p, 10)
	time.Sleep(200 * time.Millisecond)

	// Verify the old address serves metrics.
	code, body, err := scrape("http://" + addr1 + "/metrics")
	if err != nil {
		t.Fatalf("scrape addr1 before Update failed: %v", err)
	}
	if code != 200 {
		t.Fatalf("expected 200 from addr1, got %d", code)
	}
	if !strings.Contains(body, "in_octets") {
		t.Errorf("expected metric in_octets in response from addr1")
	}

	// Update with a different listen address → triggers HTTP rebuild.
	err = p.Update(context.Background(), map[string]any{
		"listen":      addr2,
		"path":        "/metrics",
		"expiration":  "60s",
		"timeout":     "10s",
		"num-workers": 2,
		"buffer-size": 1000,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	writeMessages(p, 10)
	time.Sleep(200 * time.Millisecond)

	// Old address should no longer serve.
	_, _, err = scrape("http://" + addr1 + "/metrics")
	if err == nil {
		t.Error("expected connection error on old addr1 after rebuild, but scrape succeeded")
	}

	// New address should serve metrics.
	code, body, err = scrape("http://" + addr2 + "/metrics")
	if err != nil {
		t.Fatalf("scrape addr2 after Update failed: %v", err)
	}
	if code != 200 {
		t.Fatalf("expected 200 from addr2, got %d", code)
	}
	if !strings.Contains(body, "in_octets") {
		t.Errorf("expected metric in_octets in response from addr2")
	}
}

func TestHTTPServerRebuildOnPathChange(t *testing.T) {
	addr := freeListenAddr(t)

	p := initTestOutputWithAddr(t, addr, "/metrics")
	defer p.Close()

	writeMessages(p, 10)
	time.Sleep(200 * time.Millisecond)

	// Verify the original path works.
	code, _, err := scrape("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("scrape /metrics before Update failed: %v", err)
	}
	if code != 200 {
		t.Fatalf("expected 200 from /metrics, got %d", code)
	}

	// Update with a different path → triggers HTTP rebuild.
	err = p.Update(context.Background(), map[string]any{
		"listen":      addr,
		"path":        "/prometheus",
		"expiration":  "60s",
		"timeout":     "10s",
		"num-workers": 2,
		"buffer-size": 1000,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	writeMessages(p, 10)
	time.Sleep(200 * time.Millisecond)

	// New path should serve metrics.
	code, body, err := scrape("http://" + addr + "/prometheus")
	if err != nil {
		t.Fatalf("scrape /prometheus after Update failed: %v", err)
	}
	if code != 200 {
		t.Fatalf("expected 200 from /prometheus, got %d", code)
	}
	if !strings.Contains(body, "in_octets") {
		t.Errorf("expected metric in_octets in response from /prometheus")
	}

	// Old path should return 404.
	code, _, err = scrape("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("scrape /metrics after path change failed: %v", err)
	}
	if code != 404 {
		t.Errorf("expected 404 from old /metrics path, got %d", code)
	}
}

func TestHTTPServerNoRebuildOnUnrelatedChange(t *testing.T) {
	addr := freeListenAddr(t)

	p := initTestOutputWithAddr(t, addr, "/metrics")
	defer p.Close()

	writeMessages(p, 10)
	time.Sleep(200 * time.Millisecond)

	// Verify baseline.
	code, _, err := scrape("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("scrape before Update failed: %v", err)
	}
	if code != 200 {
		t.Fatalf("expected 200, got %d", code)
	}

	// Update only num-workers (no listen/path/TLS change).
	// HTTP server should NOT be rebuilt.
	err = p.Update(context.Background(), map[string]any{
		"listen":      addr,
		"path":        "/metrics",
		"expiration":  "60s",
		"timeout":     "10s",
		"num-workers": 8,
		"buffer-size": 1000,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	writeMessages(p, 10)
	time.Sleep(200 * time.Millisecond)

	// Same address and path should still work.
	code, body, err := scrape("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("scrape after Update failed: %v", err)
	}
	if code != 200 {
		t.Fatalf("expected 200, got %d", code)
	}
	if !strings.Contains(body, "in_octets") {
		t.Errorf("expected metric in_octets in response")
	}
}

func TestDrainChanRespectsContext(t *testing.T) {
	old := make(chan int, 3)
	old <- 1
	old <- 2
	old <- 3

	// new channel with buffer 1: the first send succeeds, the second
	// would block, so the canceled context must cause drainChan to return
	// instead of hanging.
	new := make(chan int, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		drainChan(ctx, old, new)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("drainChan did not return after context cancellation")
	}
}
