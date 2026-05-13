// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package nats_output

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func memStore() store.Store[any] {
	return gomap.NewMemStore(store.StoreOptions[any]{})
}

func TestNatsOutput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[string]any
		wantErr bool
	}{
		{name: "decode buffer-size", cfg: map[string]any{"buffer-size": "x"}, wantErr: true},
		{name: "bad target-template", cfg: map[string]any{"target-template": "{{"}, wantErr: true},
		{name: "bad msg-template", cfg: map[string]any{"msg-template": "{{"}, wantErr: true},
		{name: "minimal empty cfg", cfg: map[string]any{}, wantErr: false},
	}
	n := &NatsOutput{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := n.Validate(tt.cfg)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNatsOutput_InitUpdateClose(t *testing.T) {
	n := &NatsOutput{}
	cfg := map[string]any{
		"address":           "127.0.0.1:1",
		"format":            "event",
		"buffer-size":       8,
		"connect-time-wait": "1ms",
		"num-workers":       1,
		"write-timeout":     "200ms",
	}
	if err := n.Init(context.Background(), "n1", cfg, outputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if s := n.String(); !strings.Contains(s, "127.0.0.1:1") {
		t.Fatalf("String: %s", s)
	}
	cfg2 := map[string]any{
		"address":           "127.0.0.1:1",
		"format":            "json",
		"buffer-size":       8,
		"connect-time-wait": "1ms",
		"num-workers":       1,
		"write-timeout":     "200ms",
	}
	if err := n.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	cfg3 := map[string]any{
		"address":           "127.0.0.1:2",
		"format":            "json",
		"buffer-size":       16,
		"connect-time-wait": "1ms",
		"num-workers":       1,
		"write-timeout":     "200ms",
	}
	if err := n.Update(context.Background(), cfg3); err != nil {
		t.Fatalf("Update swap: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = n.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("Close timed out")
	}
}
