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
	"text/template"
	"time"

	"github.com/openconfig/gnmic/pkg/logging"
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

		{
			name: "static headers",
			cfg: map[string]any{
				"add-headers": map[string]any{
					"env": "prod",
				},
			},
			wantErr: false,
		},
		{
			name: "templated headers",
			cfg: map[string]any{
				"add-headers": map[string]any{
					"sub": `{{ index .Meta "subscription-name" }}`,
				},
			},
			wantErr: false,
		},
		{
			name: "mixed headers",
			cfg: map[string]any{
				"add-headers": map[string]any{
					"env": "prod",
					"sub": `{{ index .Meta "subscription-name" }}`,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid header template",
			cfg: map[string]any{
				"add-headers": map[string]any{
					"sub": "{{",
				},
			},
			wantErr: true,
		},

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

func TestBuildKafkaHeaders(t *testing.T) {
	tests := []struct {
		name         string
		headers      map[string]string
		wantTplCount int
		wantErr      bool
	}{
		{
			name:         "empty headers",
			headers:      nil,
			wantTplCount: 0,
			wantErr:      false,
		},
		{
			name: "static headers only",
			headers: map[string]string{
				"env":    "prod",
				"region": "us-east-1",
			},
			wantTplCount: 2,
			wantErr:      false,
		},
		{
			name: "templated headers only",
			headers: map[string]string{
				"sub":    `{{ index .Meta "subscription-name" }}`,
				"source": `{{ index .Meta "source" }}`,
			},
			wantTplCount: 2,
			wantErr:      false,
		},
		{
			name: "mixed static and templated headers",
			headers: map[string]string{
				"env": "prod",
				"sub": `{{ index .Meta "subscription-name" }}`,
			},
			wantTplCount: 2,
			wantErr:      false,
		},
		{
			name: "invalid header template",
			headers: map[string]string{
				"sub": "{{",
			},
			wantTplCount: 0,
			wantErr:      true,
		},
		{
			name: "literal header value",
			headers: map[string]string{
				"literal": "subscription-name",
			},
			wantTplCount: 1,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headerTpls, err := buildKafkaHeaders(tt.headers)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(headerTpls) != tt.wantTplCount {
				t.Fatalf("expected %d header templates, got %d", tt.wantTplCount, len(headerTpls))
			}
		})
	}
}

func TestGetHeaders(t *testing.T) {
	tests := []struct {
		name          string
		addHeaders    map[string]string
		meta          outputs.Meta
		wantHeaders   map[string]string
		runtimeBadTpl bool
	}{
		{
			name:       "empty headers",
			addHeaders: nil,
			meta: outputs.Meta{
				"subscription-name": "interfaces",
				"source":            "router01",
			},
			wantHeaders: map[string]string{},
		},
		{
			name: "static headers only",
			addHeaders: map[string]string{
				"env":    "prod",
				"region": "us-east-1",
			},
			meta: outputs.Meta{
				"subscription-name": "interfaces",
			},
			wantHeaders: map[string]string{
				"env":    "prod",
				"region": "us-east-1",
			},
		},
		{
			name: "templated headers only",
			addHeaders: map[string]string{
				"sub":    `{{ index .Meta "subscription-name" }}`,
				"source": `{{ index .Meta "source" }}`,
			},
			meta: outputs.Meta{
				"subscription-name": "interfaces",
				"source":            "router01",
			},
			wantHeaders: map[string]string{
				"sub":    "interfaces",
				"source": "router01",
			},
		},
		{
			name: "mixed static and templated headers",
			addHeaders: map[string]string{
				"env": "prod",
				"sub": `{{ index .Meta "subscription-name" }}`,
			},
			meta: outputs.Meta{
				"subscription-name": "interfaces",
			},
			wantHeaders: map[string]string{
				"env": "prod",
				"sub": "interfaces",
			},
		},
		{
			name: "missing metadata renders empty value",
			addHeaders: map[string]string{
				"sub": `{{ index .Meta "subscription-name" }}`,
			},
			meta: outputs.Meta{},
			wantHeaders: map[string]string{
				"sub": "",
			},
		},
		{
			name: "missing key renders empty value",
			addHeaders: map[string]string{
				"sub": `{{ index .Meta "subscription-name" }}`,
			},
			meta: outputs.Meta{
				"source": "router01",
			},
			wantHeaders: map[string]string{
				"sub": "",
			},
		},
		{
			name: "runtime template error skips failed header",
			addHeaders: map[string]string{
				"env": "prod",
			},
			meta: outputs.Meta{
				"subscription-name": "interfaces",
			},
			wantHeaders: map[string]string{
				"env": "prod",
			},
			runtimeBadTpl: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &kafkaOutput{
				logger: logging.DiscardLogger(),
			}

			headerTpls, err := buildKafkaHeaders(tt.addHeaders)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.runtimeBadTpl {
				if headerTpls == nil {
					headerTpls = make(map[string]*template.Template)
				}

				// Parses successfully, but fails during execution because .Meta is not callable.
				headerTpls["bad"] = template.Must(
					template.New("bad").Parse(`{{ call .Meta }}`),
				)
			}

			dc := &dynConfig{
				headerTpls: headerTpls,
			}

			gotHeaders := k.getHeaders(dc, tt.meta)

			if len(gotHeaders) != len(tt.wantHeaders) {
				t.Fatalf("expected %d headers, got %d: %#v", len(tt.wantHeaders), len(gotHeaders), gotHeaders)
			}

			got := make(map[string]string, len(gotHeaders))
			for _, h := range gotHeaders {
				got[string(h.Key)] = string(h.Value)
			}

			for key, wantValue := range tt.wantHeaders {
				gotValue, ok := got[key]
				if !ok {
					t.Fatalf("expected header %q to exist, got headers %#v", key, got)
				}

				if gotValue != wantValue {
					t.Fatalf("header %q: expected value %q, got %q", key, wantValue, gotValue)
				}
			}
		})
	}
}
