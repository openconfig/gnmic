// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package kafka_output

import (
	"context"
	"errors"
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

func TestKafkaOutput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[string]any
		wantErr bool
	}{
		{name: "decode max-retry", cfg: map[string]any{"max-retry": "x"}, wantErr: true},
		{name: "unsupported format", cfg: map[string]any{"format": "xml"}, wantErr: true},
		{name: "required-acks", cfg: map[string]any{"required-acks": "bogus"}, wantErr: true},
		{
			name: "oauthbearer without token-url",
			cfg: map[string]any{
				"sasl": map[string]any{"mechanism": "OAUTHBEARER"},
			},
			wantErr: true,
		},
		{name: "bad target-template", cfg: map[string]any{"target-template": "{{"}, wantErr: true},
		{name: "bad msg-template", cfg: map[string]any{"msg-template": "{{"}, wantErr: true},
		{name: "valid event format", cfg: map[string]any{"format": "event"}, wantErr: false},
	}
	k := &kafkaOutput{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := k.Validate(tt.cfg)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestKafkaOutput_InitErrors(t *testing.T) {
	k := &kafkaOutput{}
	if err := k.Init(context.Background(), "k1", map[string]any{"format": "xml"}, outputs.WithConfigStore(memStore())); err == nil {
		t.Fatal("expected format error")
	}
	if err := k.Init(context.Background(), "k1", map[string]any{"kafka-version": "x"}, outputs.WithConfigStore(memStore())); err == nil {
		t.Fatal("expected createConfig error")
	}
	badOpt := func(*outputs.OutputOptions) error { return errors.New("option error") }
	if err := k.Init(context.Background(), "k1", map[string]any{"format": "event"}, badOpt); err == nil {
		t.Fatal("expected option error")
	}
}

func TestKafkaOutput_InitUpdateClose(t *testing.T) {
	k := &kafkaOutput{}
	cfg := map[string]any{
		"address":            "127.0.0.1:1",
		"format":             "event",
		"buffer-size":        8,
		"recovery-wait-time": "1ms",
		"num-workers":        1,
		"flush-frequency":    "100ms",
		"timeout":            "500ms",
	}
	if err := k.Init(context.Background(), "k1", cfg, outputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if s := k.String(); !strings.Contains(s, "127.0.0.1:1") {
		t.Fatalf("String: %s", s)
	}
	// no-op style Update (same buffer / workers)
	cfg2 := map[string]any{
		"address":            "127.0.0.1:1",
		"format":             "json",
		"buffer-size":        8,
		"recovery-wait-time": "1ms",
		"num-workers":        1,
		"flush-frequency":    "100ms",
		"timeout":            "500ms",
	}
	if err := k.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// swap channel + restart workers
	cfg3 := map[string]any{
		"address":            "127.0.0.1:2",
		"format":             "json",
		"buffer-size":        16,
		"recovery-wait-time": "1ms",
		"num-workers":        1,
		"flush-frequency":    "100ms",
		"timeout":            "500ms",
	}
	if err := k.Update(context.Background(), cfg3); err != nil {
		t.Fatalf("Update swap: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = k.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Close timed out")
	}
}
