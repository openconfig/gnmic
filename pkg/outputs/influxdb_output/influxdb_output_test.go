// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package influxdb_output

import (
	"context"
	"strings"
	"testing"

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
