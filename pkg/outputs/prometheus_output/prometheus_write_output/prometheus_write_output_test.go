// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	promcom "github.com/openconfig/gnmic/pkg/outputs/prometheus_output"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
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
		"input-buffer-size": 1,
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
	if got := cap(p.msgChan); got != 1 {
		t.Fatalf("input buffer capacity = %d, want 1", got)
	}
	cfg2 := map[string]any{
		"url":               srv.URL + "/api/v1/write",
		"interval":          "2h",
		"buffer-size":       8,
		"input-buffer-size": 1,
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
		"input-buffer-size": 1,
		"num-workers":       1,
		"num-writers":       1,
		"max-retries":       1,
		"timeout":           "500ms",
	}
	if err := p.Update(context.Background(), cfg3); err != nil {
		t.Fatalf("Update swap: %v", err)
	}
	if got := p.cfg.Load().Name; got != "pw1" {
		t.Fatalf("updated output name = %q, want pw1", got)
	}
	cfg3["input-buffer-size"] = 2
	if err := p.Update(context.Background(), cfg3); err == nil || !strings.Contains(err.Error(), "cannot be changed") {
		t.Fatalf("input buffer resize error = %v", err)
	}
	if got := cap(p.msgChan); got != 1 {
		t.Fatalf("input buffer capacity after rejected update = %d, want 1", got)
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

func newBlockedPromWriteOutput(name string, size int) *promWriteOutput {
	p := &promWriteOutput{}
	p.init()
	cfg := &config{Name: name, InputBufferSize: size}
	p.setDefaultsFor(cfg)
	p.cfg.Store(cfg)
	p.msgChan = make(chan *outputs.ProtoMsg, cfg.InputBufferSize)
	initInputQueueMetrics(name, p.msgChan)
	return p
}

func waitForBackpressure(t *testing.T, name string, before float64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if testutil.ToFloat64(prometheusWriteInputBackpressure.WithLabelValues(name)) > before {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("write did not report backpressure")
}

func TestPromWriteOutput_WritePropagatesBackpressure(t *testing.T) {
	p := newBlockedPromWriteOutput(t.Name(), 1)
	rsp := &gnmi.SubscribeResponse{}
	p.Write(context.Background(), rsp, nil)
	if got := testutil.ToFloat64(prometheusWriteInputQueueCapacity.WithLabelValues(t.Name())); got != 1 {
		t.Fatalf("input queue capacity metric = %v, want 1", got)
	}
	if got := testutil.ToFloat64(prometheusWriteInputQueueDepth.WithLabelValues(t.Name())); got != 1 {
		t.Fatalf("input queue depth metric = %v, want 1", got)
	}

	histogram := prometheusWriteInputBackpressureDuration.WithLabelValues(t.Name()).(prometheus.Histogram)
	beforeMetric := &dto.Metric{}
	if err := histogram.Write(beforeMetric); err != nil {
		t.Fatal(err)
	}
	blocked := prometheusWriteInputBackpressure.WithLabelValues(t.Name())
	before := testutil.ToFloat64(blocked)
	done := make(chan struct{})
	go func() {
		p.Write(context.Background(), rsp, nil)
		close(done)
	}()
	waitForBackpressure(t, t.Name(), before)
	select {
	case <-done:
		t.Fatal("write returned while the input queue was full")
	default:
	}

	time.Sleep(25 * time.Millisecond)
	<-p.msgChan
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("write did not resume after input capacity became available")
	}
	if got := len(p.msgChan); got != 1 {
		t.Fatalf("input queue depth = %d, want 1", got)
	}
	afterMetric := &dto.Metric{}
	if err := histogram.Write(afterMetric); err != nil {
		t.Fatal(err)
	}
	elapsed := afterMetric.GetHistogram().GetSampleSum() - beforeMetric.GetHistogram().GetSampleSum()
	if elapsed < 0.020 {
		t.Fatalf("backpressure duration = %fs, want at least 0.020s", elapsed)
	}
}

func TestPromWriteOutput_CancellationReleasesBackpressure(t *testing.T) {
	p := newBlockedPromWriteOutput(t.Name(), 1)
	rsp := &gnmi.SubscribeResponse{}
	p.Write(context.Background(), rsp, nil)

	blocked := prometheusWriteInputBackpressure.WithLabelValues(t.Name())
	before := testutil.ToFloat64(blocked)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Write(ctx, rsp, nil)
		close(done)
	}()
	waitForBackpressure(t, t.Name(), before)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled write remained blocked")
	}
	if got := len(p.msgChan); got != 1 {
		t.Fatalf("input queue depth = %d, want 1", got)
	}
}

func TestPromWriteOutput_CloseReleasesBackpressure(t *testing.T) {
	p := newBlockedPromWriteOutput(t.Name(), 1)
	rsp := &gnmi.SubscribeResponse{}
	p.Write(context.Background(), rsp, nil)

	blocked := prometheusWriteInputBackpressure.WithLabelValues(t.Name())
	before := testutil.ToFloat64(blocked)
	writeDone := make(chan struct{})
	go func() {
		p.Write(context.Background(), rsp, nil)
		close(writeDone)
	}()
	waitForBackpressure(t, t.Name(), before)
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("output close did not release blocked write")
	}
}

func TestPromWriteOutput_CanceledWriteIsNotQueued(t *testing.T) {
	p := newBlockedPromWriteOutput(t.Name(), 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.Write(ctx, &gnmi.SubscribeResponse{}, nil)
	if got := len(p.msgChan); got != 0 {
		t.Fatalf("input queue depth = %d, want 0", got)
	}
}

func TestPromWriteOutput_WorkerCancellationReleasesFullTimeSeriesBuffer(t *testing.T) {
	p := &promWriteOutput{}
	p.init()
	cfg := &config{Name: t.Name(), BufferSize: 1}
	p.setDefaultsFor(cfg)
	p.cfg.Store(cfg)
	p.dynCfg.Store(&dynConfig{mb: &promcom.MetricBuilder{}})
	timeSeries := make(chan *prompb.TimeSeries, 1)
	timeSeries <- &prompb.TimeSeries{}
	p.timeSeriesCh.Store(&timeSeries)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.workerHandleEvent(ctx, &formatters.EventMsg{
			Name:      "metric",
			Timestamp: 1,
			Values:    map[string]any{"value": 1},
		})
		close(done)
	}()
	select {
	case <-p.buffDrainCh:
	case <-time.After(time.Second):
		t.Fatal("worker did not reach the full time-series buffer")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker remained blocked after cancellation")
	}
}
