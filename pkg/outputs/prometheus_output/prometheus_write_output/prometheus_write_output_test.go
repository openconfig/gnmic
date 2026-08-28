// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func memStore() store.Store[any] {
	return gomap.NewMemStore(store.StoreOptions[any]{})
}

func TestPromWriteOutput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[string]any
		wantErr bool
	}{
		{name: "decode buffer-size", cfg: map[string]any{"buffer-size": "x"}, wantErr: true},
		{name: "missing url", cfg: map[string]any{}, wantErr: true},
		{name: "bad url", cfg: map[string]any{"url": "://bad"}, wantErr: true},
		{
			name: "bad target-template",
			cfg: map[string]any{
				"url":             "http://localhost:9090",
				"target-template": "{{",
			},
			wantErr: true,
		},
		{name: "valid minimal", cfg: map[string]any{"url": "http://localhost:9090"}, wantErr: false},
	}
	p := &promWriteOutput{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.Validate(tt.cfg)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPromWriteOutput_InitUpdateClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := &promWriteOutput{}
	cfg := map[string]any{
		"url":               srv.URL + "/api/v1/write",
		"interval":          "1h",
		"buffer-size":       8,
		"input-buffer-size": 4,
		"num-workers":       1,
		"num-writers":       1,
		"max-retries":       1,
		"timeout":           "500ms",
	}
	if err := p.Init(context.Background(), "pw1", cfg, outputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if s := p.String(); !strings.Contains(s, srv.URL) {
		t.Fatalf("String: %s", s)
	}
	if got := cap(p.msgChan); got != 4 {
		t.Fatalf("input buffer capacity = %d, want 4", got)
	}
	cfg2 := map[string]any{
		"url":               srv.URL + "/api/v1/write",
		"interval":          "2h",
		"buffer-size":       8,
		"input-buffer-size": 4,
		"num-workers":       1,
		"num-writers":       1,
		"max-retries":       1,
		"timeout":           "500ms",
	}
	if err := p.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	cfg3 := map[string]any{
		"url":               srv.URL + "/api/v1/write",
		"interval":          "2h",
		"buffer-size":       16,
		"input-buffer-size": 12,
		"num-workers":       1,
		"num-writers":       1,
		"max-retries":       1,
		"timeout":           "500ms",
	}
	if err := p.Update(context.Background(), cfg3); err != nil {
		t.Fatalf("Update swap: %v", err)
	}
	if got := cap(p.msgChan); got != 12 {
		t.Fatalf("updated input buffer capacity = %d, want 12", got)
	}
	if got := p.cfg.Load().Name; got != "pw1" {
		t.Fatalf("updated output name = %q, want pw1", got)
	}
	done := make(chan struct{})
	go func() {
		_ = p.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Close timed out")
	}
}

func TestPromWriteOutput_InitErrors(t *testing.T) {
	p := &promWriteOutput{}
	if err := p.Init(context.Background(), "pw1", map[string]any{}, outputs.WithConfigStore(memStore())); err == nil {
		t.Fatal("expected missing url")
	}
}

func TestPromWriteOutput_InputBufferIsBounded(t *testing.T) {
	p := &promWriteOutput{}
	p.init()
	cfg := &config{Name: t.Name(), InputBufferSize: 2}
	p.setDefaultsFor(cfg)
	p.cfg.Store(cfg)
	p.msgChan = make(chan *outputs.ProtoMsg, cfg.InputBufferSize)
	setInputQueueMetrics(cfg.Name, p.msgChan)

	dropped := prometheusWriteInputMessagesDropped.WithLabelValues(cfg.Name, inputDropReasonBufferFull)
	before := testutil.ToFloat64(dropped)
	rsp := &gnmi.SubscribeResponse{}

	p.Write(context.Background(), rsp, nil)
	p.Write(context.Background(), rsp, nil)
	writeDone := make(chan struct{})
	go func() {
		p.Write(context.Background(), rsp, nil)
		close(writeDone)
	}()

	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("Write blocked with a full input buffer")
	}
	if got := len(p.msgChan); got != cfg.InputBufferSize {
		t.Fatalf("input queue depth = %d, want %d", got, cfg.InputBufferSize)
	}
	if got := testutil.ToFloat64(dropped) - before; got != 1 {
		t.Fatalf("dropped messages = %v, want 1", got)
	}
	if got := testutil.ToFloat64(prometheusWriteInputQueueDepth.WithLabelValues(cfg.Name)); got != 2 {
		t.Fatalf("input queue depth metric = %v, want 2", got)
	}
	if got := testutil.ToFloat64(prometheusWriteInputQueueCapacity.WithLabelValues(cfg.Name)); got != 2 {
		t.Fatalf("input queue capacity metric = %v, want 2", got)
	}
}

func TestPromWriteOutput_CanceledWriteIsNotCountedAsDropped(t *testing.T) {
	p := &promWriteOutput{}
	p.init()
	cfg := &config{Name: t.Name(), InputBufferSize: 1}
	p.setDefaultsFor(cfg)
	p.cfg.Store(cfg)
	p.msgChan = make(chan *outputs.ProtoMsg, cfg.InputBufferSize)

	dropped := prometheusWriteInputMessagesDropped.WithLabelValues(cfg.Name, inputDropReasonBufferFull)
	before := testutil.ToFloat64(dropped)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.Write(ctx, &gnmi.SubscribeResponse{}, nil)

	if got := testutil.ToFloat64(dropped) - before; got != 0 {
		t.Fatalf("dropped messages = %v, want 0 for a canceled context", got)
	}
	if got := len(p.msgChan); got != 0 {
		t.Fatalf("input queue depth = %d, want 0", got)
	}
}

func TestPromWriteOutput_DrainInputMessagesIsBounded(t *testing.T) {
	p := &promWriteOutput{}
	oldChan := make(chan *outputs.ProtoMsg, 3)
	newChan := make(chan *outputs.ProtoMsg, 2)
	for range 3 {
		oldChan <- outputs.NewProtoMsg(&gnmi.SubscribeResponse{}, nil)
	}

	dropped := prometheusWriteInputMessagesDropped.WithLabelValues(t.Name(), inputDropReasonBufferResize)
	before := testutil.ToFloat64(dropped)
	p.drainInputMessages(oldChan, newChan, t.Name())

	if got := len(newChan); got != cap(newChan) {
		t.Fatalf("new input queue depth = %d, want %d", got, cap(newChan))
	}
	if got := len(oldChan); got != 0 {
		t.Fatalf("old input queue depth = %d, want 0", got)
	}
	if got := testutil.ToFloat64(dropped) - before; got != 1 {
		t.Fatalf("resize drops = %v, want 1", got)
	}
}

func TestPromWriteOutput_ConcurrentWriteAndInputBufferResize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := &promWriteOutput{}
	baseConfig := func(inputBufferSize int) map[string]any {
		return map[string]any{
			"url":                       srv.URL + "/api/v1/write",
			"interval":                  "10ms",
			"buffer-size":               32,
			"input-buffer-size":         inputBufferSize,
			"max-time-series-per-write": 16,
			"num-workers":               1,
			"num-writers":               1,
			"metadata":                  map[string]any{"include": false},
		}
	}
	if err := p.Init(context.Background(), t.Name(), baseConfig(4), outputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}

	var writers sync.WaitGroup
	for range 8 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range 200 {
				p.Write(context.Background(), &gnmi.SubscribeResponse{}, nil)
			}
		}()
	}
	for _, size := range []int{8, 2, 16, 4, 12} {
		if err := p.Update(context.Background(), baseConfig(size)); err != nil {
			t.Fatalf("Update input-buffer-size=%d: %v", size, err)
		}
	}
	writers.Wait()

	if got := cap(p.msgChan); got != 12 {
		t.Fatalf("final input buffer capacity = %d, want 12", got)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
