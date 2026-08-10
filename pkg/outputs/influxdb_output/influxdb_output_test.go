// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package influxdb_output

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/formatters"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func memStore() store.Store[any] {
	return gomap.NewMemStore(store.StoreOptions[any]{})
}

func TestInfluxDBOutput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[string]any
		wantErr bool
	}{
		{name: "decode batch-size", cfg: map[string]any{"batch-size": "x"}, wantErr: true},
		{name: "bad url", cfg: map[string]any{"url": "://bad"}, wantErr: true},
		{name: "bad target-template", cfg: map[string]any{"target-template": "{{"}, wantErr: true},
		{name: "valid minimal url", cfg: map[string]any{"url": "http://localhost:8086"}, wantErr: false},
	}
	i := &influxDBOutput{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := i.Validate(tt.cfg)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInfluxDBOutput_InitUpdateClose(t *testing.T) {
	i := &influxDBOutput{}
	cfg := map[string]any{
		"url":                 "http://127.0.0.1:9",
		"org":                 "o",
		"bucket":              "b",
		"token":               "t",
		"health-check-period": "0",
		"flush-timer":         "1h",
		"batch-size":          10,
	}
	if err := i.Init(context.Background(), "in1", cfg, outputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if s := i.String(); !strings.Contains(s, "127.0.0.1:9") {
		t.Fatalf("String: %s", s)
	}
	cfg2 := map[string]any{
		"url":                 "http://127.0.0.1:9",
		"org":                 "o2",
		"bucket":              "b2",
		"token":               "t2",
		"health-check-period": "0",
		"flush-timer":         "2h",
		"batch-size":          10,
	}
	if err := i.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := i.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestInfluxDBOutput_InitDecodeError(t *testing.T) {
	i := &influxDBOutput{}
	if err := i.Init(context.Background(), "in1", map[string]any{"batch-size": "x"}, outputs.WithConfigStore(memStore())); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClientOptsFor(t *testing.T) {
	_, err := clientOptsFor(&Config{
		UseGzip:            true,
		TimestampPrecision: "ms",
		Debug:              true,
		EnableTLS:          true,
	})
	if err != nil {
		t.Fatalf("clientOptsFor: %v", err)
	}
	if _, err := clientOptsFor(&Config{TLS: &types.TLSConfig{CaFile: "/no/such/file.pem"}}); err == nil {
		t.Fatal("expected TLS config error")
	}
}

// TestInfluxDBOutput_UpdateUnderConcurrentWrites is a regression test for the
// use-after-close on client rebuild: Update() used to close the old influx
// client immediately after swapping the pointer, while workers were still
// holding it and calling client.WriteAPI(). Run with -race.
func TestInfluxDBOutput_UpdateUnderConcurrentWrites(t *testing.T) {
	i := &influxDBOutput{}
	base := func(org string) map[string]any {
		return map[string]any{
			"url":                 "http://127.0.0.1:9",
			"org":                 org,
			"bucket":              "b",
			"token":               "t",
			"health-check-period": "0",
			"flush-timer":         "1h",
			"batch-size":          10,
		}
	}
	if err := i.Init(context.Background(), "in1", base("o0"),
		outputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = i.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// writers feeding the worker while the client is rebuilt underneath them
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				i.WriteEvent(ctx, &formatters.EventMsg{
					Name:      "m",
					Timestamp: time.Now().UnixNano(),
					Tags:      map[string]string{"t": "v"},
					Values:    map[string]any{"v": 1},
				})
			}
		}()
	}

	// force a client rebuild on every Update by changing the token
	for n := 0; n < 12; n++ {
		cfg := base("o0")
		cfg["token"] = fmt.Sprintf("tok%d", n)
		if err := i.Update(context.Background(), cfg); err != nil {
			cancel()
			wg.Wait()
			t.Fatalf("Update %d: %v", n, err)
		}
	}

	cancel()
	wg.Wait()
}

// Close() waits on the worker WaitGroup, so every blocking point inside a
// worker must observe context cancellation. A worker parked waiting for health
// recovery previously had an unconditional receive on the startSig channel,
// which would hang shutdown until influx came back.
func TestInfluxDBOutput_CloseReturnsWhileWorkerAwaitsRecovery(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"influxdb","status":"pass","version":"2.0.0"}`))
	}))
	t.Cleanup(srv.Close)

	i := &influxDBOutput{}
	cfg := map[string]any{
		"url":                 srv.URL,
		"org":                 "o",
		"bucket":              "b",
		"token":               "t",
		"health-check-period": "50ms",
		"flush-timer":         "1h",
		"batch-size":          10,
	}
	if err := i.Init(context.Background(), "in2", cfg,
		outputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Take influx away so the health check fails and the worker parks on
	// startSig waiting for recovery.
	healthy.Store(false)
	time.Sleep(300 * time.Millisecond)

	done := make(chan struct{})
	go func() { _ = i.Close(); close(done) }()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Close() hung waiting for a worker parked on startSig")
	}
}

// Init used to retry the health probe in a `goto CRCLIENT` loop with a 2s
// sleep, so it never returned while influx was unreachable and gnmic hung at
// startup. It must now start and let the health check goroutine recover.
func TestInfluxDBOutput_InitDoesNotBlockOnDeadServer(t *testing.T) {
	i := &influxDBOutput{}
	cfg := map[string]any{
		"url":                 "http://127.0.0.1:9",
		"org":                 "o",
		"bucket":              "b",
		"token":               "t",
		"health-check-period": "50ms",
		"flush-timer":         "1h",
		"batch-size":          10,
	}

	done := make(chan error, 1)
	go func() {
		done <- i.Init(context.Background(), "dead", cfg,
			outputs.WithConfigStore(memStore()))
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Init returned an error instead of starting: %v", err)
		}
		t.Cleanup(func() { _ = i.Close() })
	case <-time.After(10 * time.Second):
		t.Fatal("Init blocked while the influx server was unreachable")
	}
}
