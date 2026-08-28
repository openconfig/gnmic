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
	"github.com/openconfig/gnmic/pkg/outputs"
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
			name: "bad message sampling interval",
			cfg: map[string]any{
				"url": "http://localhost:9090",
				"message-sampling": map[string]any{
					"by-subscription": map[string]any{
						"counters": map[string]any{"interval": "invalid"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid message sampling",
			cfg: map[string]any{
				"url": "http://localhost:9090",
				"message-sampling": map[string]any{
					"by-subscription": map[string]any{
						"counters": map[string]any{
							"interval":      "15s",
							"minimum-bytes": 262144,
						},
					},
					"cache-size": 1000,
				},
			},
			wantErr: false,
		},
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
		"url":         srv.URL + "/api/v1/write",
		"interval":    "1h",
		"buffer-size": 8,
		"num-workers": 1,
		"num-writers": 1,
		"max-retries": 1,
		"timeout":     "500ms",
	}
	if err := p.Init(context.Background(), "pw1", cfg, outputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if s := p.String(); !strings.Contains(s, srv.URL) {
		t.Fatalf("String: %s", s)
	}
	cfg2 := map[string]any{
		"url":         srv.URL + "/api/v1/write",
		"interval":    "2h",
		"buffer-size": 8,
		"num-workers": 1,
		"num-writers": 1,
		"max-retries": 1,
		"timeout":     "500ms",
	}
	if err := p.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	cfg3 := map[string]any{
		"url":         srv.URL + "/api/v1/write",
		"interval":    "2h",
		"buffer-size": 16,
		"num-workers": 1,
		"num-writers": 1,
		"max-retries": 1,
		"timeout":     "500ms",
	}
	if err := p.Update(context.Background(), cfg3); err != nil {
		t.Fatalf("Update swap: %v", err)
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

func TestPromWriteOutputSamplingSkipsBeforeInputQueue(t *testing.T) {
	p := &promWriteOutput{}
	p.init()
	p.cfg.Store(&config{Name: "pw1", Timeout: time.Second})
	sampler, err := newSubscriptionSampler(&messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	if err != nil {
		t.Fatalf("newSubscriptionSampler() error = %v", err)
	}
	p.sampler.Store(sampler)
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	if !sampler.Allow(meta, 0, time.Now()) {
		t.Fatal("failed to seed sampler")
	}

	done := make(chan struct{})
	go func() {
		p.Write(context.Background(), &gnmi.SubscribeResponse{}, meta)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("sampled message blocked on input queue")
	}
}

func TestPromWriteOutputSamplingPreservesNarrowMessages(t *testing.T) {
	p := &promWriteOutput{}
	p.init()
	p.cfg.Store(&config{Name: "pw1", Timeout: time.Second})
	sampler, err := newSubscriptionSampler(&messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute, MinimumBytes: 1024},
		},
	})
	if err != nil {
		t.Fatalf("newSubscriptionSampler() error = %v", err)
	}
	p.sampler.Store(sampler)
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	if !sampler.Allow(meta, 2048, time.Now()) {
		t.Fatal("failed to seed sampler")
	}

	received := make(chan struct{})
	go func() {
		<-p.msgChan
		close(received)
	}()
	p.Write(context.Background(), &gnmi.SubscribeResponse{}, meta)
	select {
	case <-received:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("narrow message did not bypass sampling")
	}
}
