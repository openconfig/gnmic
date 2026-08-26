// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package tcp_output

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

type shortWriter struct {
	bytes.Buffer
}

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return w.Buffer.Write(p)
}

type partialErrorConn struct {
	n int
}

func (c *partialErrorConn) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (c *partialErrorConn) Write(p []byte) (int, error) {
	n := min(c.n, len(p))
	return n, errors.New("forced write failure")
}

func (*partialErrorConn) Close() error {
	return nil
}

func (*partialErrorConn) LocalAddr() net.Addr {
	return nil
}

func (*partialErrorConn) RemoteAddr() net.Addr {
	return nil
}

func (*partialErrorConn) SetDeadline(time.Time) error {
	return nil
}

func (*partialErrorConn) SetReadDeadline(time.Time) error {
	return nil
}

func (*partialErrorConn) SetWriteDeadline(time.Time) error {
	return nil
}

func newStore() store.Store[any] {
	return gomap.NewMemStore(store.StoreOptions[any]{})
}

func freeTCPListener(t *testing.T) (string, func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 4096)
				for {
					if _, err := c.Read(buf); err != nil {
						c.Close()
						return
					}
				}
			}(c)
		}
	}()
	return l.Addr().String(), func() { l.Close() }
}

func TestTCP_SetDefaults(t *testing.T) {
	c := &config{}
	setDefaultsFor(c)
	if c.RetryInterval != defaultRetryTimer {
		t.Errorf("retry interval default")
	}
	if c.NumWorkers != defaultNumWorkers {
		t.Errorf("num workers default")
	}
	if c.MaxRetries != defaultMaxRetries {
		t.Errorf("max retries default = %d, want %d", c.MaxRetries, defaultMaxRetries)
	}
}

func TestTCP_Validate(t *testing.T) {
	t1 := &tcpOutput{}
	if err := t1.Validate(map[string]any{}); err == nil {
		t.Errorf("expected missing address error")
	}
	if err := t1.Validate(map[string]any{"address": "bad"}); err == nil {
		t.Errorf("expected bad address")
	}
	// missing target-template
	if err := t1.Validate(map[string]any{"address": "127.0.0.1:1"}); err == nil {
		t.Errorf("expected target-template error")
	}
	if err := t1.Validate(map[string]any{
		"address":         "127.0.0.1:1",
		"target-template": "foo",
	}); err != nil {
		t.Errorf("valid: %v", err)
	}
	// decode failure
	if err := t1.Validate(map[string]any{"buffer-size": "x"}); err == nil {
		t.Errorf("expected decode error")
	}
	if err := t1.Validate(map[string]any{
		"address":         "127.0.0.1:1",
		"target-template": "foo",
		"max-retries":     -1,
	}); err == nil {
		t.Errorf("expected negative max-retries error")
	}
}

func TestTCP_InitAndUpdate(t *testing.T) {
	addr, stop := freeTCPListener(t)
	defer stop()

	o := &tcpOutput{}
	cfg := map[string]any{
		"address":         addr,
		"format":          "event",
		"buffer-size":     16,
		"rate":            "10ms",
		"delimiter":       "\n",
		"num-workers":     1,
		"target-template": "{{ .target }}",
	}
	if err := o.Init(context.Background(), "tcp1", cfg, outputs.WithConfigStore(newStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer o.Close()

	if !strings.Contains(o.String(), addr) {
		t.Errorf("String missing address: %s", o.String())
	}

	// Non-restarting Update: change format/rate/delimiter only.
	cfg2 := map[string]any{
		"address":         addr,
		"format":          "json",
		"buffer-size":     16,
		"rate":            "20ms",
		"delimiter":       "|",
		"num-workers":     1,
		"target-template": "{{ .target }}",
	}
	if err := o.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update no-op: %v", err)
	}
	// Update changing buffer-size triggers swap + restart.
	cfg3 := map[string]any{
		"address":         addr,
		"format":          "json",
		"buffer-size":     32,
		"num-workers":     1,
		"target-template": "{{ .target }}",
	}
	if err := o.Update(context.Background(), cfg3); err != nil {
		t.Fatalf("Update swap: %v", err)
	}
	// Update changing num-workers triggers restart only.
	cfg4 := map[string]any{
		"address":         addr,
		"format":          "json",
		"buffer-size":     32,
		"num-workers":     2,
		"target-template": "{{ .target }}",
	}
	if err := o.Update(context.Background(), cfg4); err != nil {
		t.Fatalf("Update restart: %v", err)
	}
	// Decode error.
	if err := o.Update(context.Background(), map[string]any{"buffer-size": "x"}); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestTCP_InitErrors(t *testing.T) {
	o := &tcpOutput{}
	if err := o.Init(context.Background(), "tcp1", map[string]any{
		"address": "bad",
	}, outputs.WithConfigStore(newStore())); err == nil {
		t.Errorf("expected bad address")
	}
	o = &tcpOutput{}
	if err := o.Init(context.Background(), "tcp1", map[string]any{
		"buffer-size": "x",
	}, outputs.WithConfigStore(newStore())); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestTCP_WritePayloadHandlesShortWrites(t *testing.T) {
	writer := new(shortWriter)
	want := []byte("telemetry")

	if err := writeTCPPayload(writer, want); err != nil {
		t.Fatalf("writeTCPPayload() error = %v", err)
	}
	if got := writer.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("writeTCPPayload() wrote %q, want %q", got, want)
	}
}

func TestTCP_RetriesMessageAfterWriteFailure(t *testing.T) {
	o := &tcpOutput{}
	o.init()

	cfg := &config{
		Address:       "127.0.0.1:1",
		RetryInterval: time.Millisecond,
		MaxRetries:    defaultMaxRetries,
	}
	o.cfg.Store(cfg)
	o.dynCfg.Store(&dynConfig{delimiter: []byte("\n")})

	buffer := make(chan []byte, 2)
	o.buffer.Store(&buffer)

	secondClient, secondServer := net.Pipe()
	defer secondServer.Close()

	connections := make(chan net.Conn, 2)
	connections <- &partialErrorConn{n: 2}
	connections <- secondClient

	var dialCount atomic.Int32
	dial := func(ctx context.Context, _ string) (net.Conn, error) {
		dialCount.Add(1)
		select {
		case conn := <-connections:
			return conn, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go o.startWithDialer(ctx, wg, 0, dial)

	buffer <- []byte("first")
	buffer <- []byte("second")

	want := []byte("first\nsecond\n")
	got := make([]byte, len(want))
	if err := secondServer.SetReadDeadline(
		time.Now().Add(5 * time.Second),
	); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	if _, err := io.ReadFull(secondServer, got); err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("received %q, want %q", got, want)
	}

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("TCP worker did not stop after cancellation")
	}

	if got := dialCount.Load(); got != 2 {
		t.Fatalf("dial count = %d, want 2", got)
	}
}

func TestTCP_RegisterMetrics(t *testing.T) {
	o := &tcpOutput{}
	o.init()
	o.name = t.Name()
	o.reg = prometheus.NewRegistry()
	cfg := &config{EnableMetrics: true}
	o.cfg.Store(cfg)

	if err := o.registerMetrics(cfg); err != nil {
		t.Fatalf("registerMetrics() error = %v", err)
	}

	families, err := o.reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	got := make(map[string]bool, len(families))
	for _, family := range families {
		got[family.GetName()] = true
	}

	for _, want := range []string{
		"gnmic_tcp_output_errors_total",
		"gnmic_tcp_output_dropped_messages_total",
	} {
		if !got[want] {
			t.Errorf("registered metrics missing %q", want)
		}
	}
}

func TestTCP_WriteDoesNotBlockWhenBufferIsFull(t *testing.T) {
	o := &tcpOutput{}
	o.init()
	o.name = t.Name()
	cfg := &config{EnableMetrics: true, Format: "protojson"}
	o.cfg.Store(cfg)
	o.dynCfg.Store(&dynConfig{
		mo: &formatters.MarshalOptions{Format: cfg.Format},
	})
	buffer := make(chan []byte, 1)
	buffer <- []byte("queued")
	o.buffer.Store(&buffer)
	droppedBefore := testutil.ToFloat64(
		tcpOutputDroppedMessages.WithLabelValues(o.name, "buffer_full"),
	)

	done := make(chan struct{})
	go func() {
		o.Write(context.Background(), &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true},
		}, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		<-buffer
		<-done
		t.Fatal("full TCP output buffer blocked the caller")
	}
	if got := len(buffer); got != 1 {
		t.Fatalf("buffer length = %d, want 1", got)
	}

	if got := testutil.ToFloat64(
		tcpOutputDroppedMessages.WithLabelValues(o.name, "buffer_full"),
	) - droppedBefore; got != 1 {
		t.Fatalf("buffer-full dropped metric delta = %v, want 1", got)
	}
}

func TestTCP_EnqueueReturnsWhenContextIsCanceled(t *testing.T) {
	o := &tcpOutput{}
	o.init()
	o.name = t.Name()
	cfg := &config{EnableMetrics: true}
	buffer := make(chan []byte, 1)
	buffer <- []byte("queued")
	droppedBefore := testutil.ToFloat64(
		tcpOutputDroppedMessages.WithLabelValues(o.name, "buffer_full"),
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	o.enqueue(ctx, cfg, buffer, []byte("new"))

	if got := len(buffer); got != 1 {
		t.Fatalf("buffer length = %d, want 1", got)
	}
	if got := testutil.ToFloat64(
		tcpOutputDroppedMessages.WithLabelValues(o.name, "buffer_full"),
	) - droppedBefore; got != 0 {
		t.Fatalf("buffer-full dropped metric delta = %v, want 0", got)
	}
}

func TestTCP_UpdateEnablesMetrics(t *testing.T) {
	addr, stop := freeTCPListener(t)
	defer stop()

	o := &tcpOutput{}
	reg := prometheus.NewRegistry()
	cfg := map[string]any{
		"address":         addr,
		"format":          "json",
		"buffer-size":     16,
		"num-workers":     1,
		"target-template": "{{ .target }}",
		"enable-metrics":  false,
	}
	if err := o.Init(
		context.Background(),
		t.Name(),
		cfg,
		outputs.WithConfigStore(newStore()),
		outputs.WithRegistry(reg),
	); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer o.Close()

	cfg["enable-metrics"] = true
	if err := o.Update(context.Background(), cfg); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	got := make(map[string]bool, len(families))
	for _, family := range families {
		got[family.GetName()] = true
	}
	for _, want := range []string{
		"gnmic_tcp_output_errors_total",
		"gnmic_tcp_output_dropped_messages_total",
	} {
		if !got[want] {
			t.Errorf("metrics after Update missing %q", want)
		}
	}
}

func TestTCP_DropsMessageAfterRetryLimit(t *testing.T) {
	o := &tcpOutput{}
	o.init()
	o.name = t.Name()

	cfg := &config{
		Address:       "127.0.0.1:1",
		RetryInterval: time.Millisecond,
		MaxRetries:    2,
		EnableMetrics: true,
	}
	o.cfg.Store(cfg)
	o.dynCfg.Store(&dynConfig{delimiter: []byte("\n")})

	buffer := make(chan []byte, 2)
	o.buffer.Store(&buffer)

	successClient, successServer := net.Pipe()
	defer successServer.Close()

	var dialCount atomic.Int32
	dial := func(ctx context.Context, _ string) (net.Conn, error) {
		switch dialCount.Add(1) {
		case 1:
			return &partialErrorConn{n: 0}, nil
		case 2, 3:
			return nil, errors.New("forced dial failure")
		case 4:
			return successClient, nil
		default:
			<-ctx.Done()
			return nil, ctx.Err()
		}
	}

	writeErrorsBefore := testutil.ToFloat64(
		tcpOutputErrors.WithLabelValues(o.name, "write"),
	)
	dialErrorsBefore := testutil.ToFloat64(
		tcpOutputErrors.WithLabelValues(o.name, "dial"),
	)
	droppedBefore := testutil.ToFloat64(
		tcpOutputDroppedMessages.WithLabelValues(o.name, "max_retries"),
	)

	ctx, cancel := context.WithCancel(context.Background())
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go o.startWithDialer(ctx, wg, 0, dial)

	buffer <- []byte("first")
	buffer <- []byte("second")

	want := []byte("second\n")
	got := make([]byte, len(want))
	if err := successServer.SetReadDeadline(
		time.Now().Add(5 * time.Second),
	); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	if _, err := io.ReadFull(successServer, got); err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("received %q, want %q", got, want)
	}

	cancel()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("TCP worker did not stop after cancellation")
	}

	if got := dialCount.Load(); got != 4 {
		t.Fatalf("dial count = %d, want 4", got)
	}

	if got := testutil.ToFloat64(
		tcpOutputErrors.WithLabelValues(o.name, "write"),
	) - writeErrorsBefore; got != 1 {
		t.Errorf("write error metric delta = %v, want 1", got)
	}
	if got := testutil.ToFloat64(
		tcpOutputErrors.WithLabelValues(o.name, "dial"),
	) - dialErrorsBefore; got != 2 {
		t.Errorf("dial error metric delta = %v, want 2", got)
	}
	if got := testutil.ToFloat64(
		tcpOutputDroppedMessages.WithLabelValues(o.name, "max_retries"),
	) - droppedBefore; got != 1 {
		t.Errorf("dropped metric delta = %v, want 1", got)
	}
}

func TestTCP_Predicates(t *testing.T) {
	a := &config{BufferSize: 1, NumWorkers: 1}
	b := &config{BufferSize: 2, NumWorkers: 1}
	if !channelNeedsSwap(a, b) {
		t.Errorf("swap on bs change")
	}
	if channelNeedsSwap(a, a) {
		t.Errorf("no swap same")
	}
	if !channelNeedsSwap(nil, a) {
		t.Errorf("swap on nil")
	}
	c := &config{NumWorkers: 2}
	if !needsWorkerRestart(a, c) {
		t.Errorf("restart on workers change")
	}
	if !needsWorkerRestart(nil, a) {
		t.Errorf("restart on nil")
	}
}
