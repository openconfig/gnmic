// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	zstore "github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func newApplyTestServer(t *testing.T) *Server {
	t.Helper()
	st := collstore.NewStore(gomap.NewMemStore(zstore.StoreOptions[any]{}))
	return NewServer(st, nil, nil, nil, nil, prometheus.NewRegistry())
}

func postConfigApply(t *testing.T, s *Server, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/apply", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleConfigApply(w, req)
	return w.Result()
}

// Regression: /config/apply must not delete tunnel-managed targets when the
// request body omits targets (Go apply.go skips entries with TunnelTargetType set).
func TestHandleConfigApply_preservesTunnelTargets(t *testing.T) {
	s := newApplyTestServer(t)

	static := &types.TargetConfig{
		Name:    "router1",
		Address: "10.0.0.1:57400",
	}
	if _, err := s.store.Config.Set("targets", static.Name, static); err != nil {
		t.Fatalf("seed static target: %v", err)
	}

	tunnel := &types.TargetConfig{
		Name:             "srl1",
		TunnelTargetType: "GNMI_GNOI",
		Subscriptions:    []string{"ifaces"},
	}
	if _, err := s.store.Config.Set("targets", tunnel.Name, tunnel); err != nil {
		t.Fatalf("seed tunnel target: %v", err)
	}

	applyBody := `{
		"subscriptions": {
			"ifaces": {
				"name": "ifaces",
				"paths": ["/interfaces/interface/state/counters"],
				"mode": "STREAM",
				"stream-mode": "TARGET_DEFINED"
			}
		}
	}`
	resp := postConfigApply(t, s, applyBody)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("apply status %d", resp.StatusCode)
	}

	if _, ok, err := s.store.Config.Get("targets", "router1"); err != nil {
		t.Fatalf("get static target: %v", err)
	} else if ok {
		t.Fatal("expected static target router1 to be deleted when omitted from apply body")
	}

	got, ok, err := s.store.Config.Get("targets", "srl1")
	if err != nil {
		t.Fatalf("get tunnel target: %v", err)
	}
	if !ok {
		t.Fatal("expected tunnel target srl1 to be preserved when omitted from apply body")
	}
	tc, ok := got.(*types.TargetConfig)
	if !ok {
		t.Fatalf("tunnel target type: %T", got)
	}
	if tc.TunnelTargetType != "GNMI_GNOI" {
		t.Fatalf("tunnel-target-type: got %q want GNMI_GNOI", tc.TunnelTargetType)
	}
}

func TestValidateApplyRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *ConfigApplyRequest
		wantErr string
	}{
		{
			name:    "empty reset",
			req:     &ConfigApplyRequest{},
			wantErr: "",
		},
		{
			name: "targets without subscriptions",
			req: &ConfigApplyRequest{
				Targets: map[string]*types.TargetConfig{"t": {Name: "t"}},
			},
			wantErr: "at least one subscription is required",
		},
		{
			name: "tunnel matches without subscriptions",
			req: &ConfigApplyRequest{
				TunnelTargetMatches: map[string]*config.TunnelTargetMatch{
					"m1": {ID: "m1", Type: "regex"},
				},
			},
			wantErr: "if tunnel-target-matches are provided",
		},
		{
			name: "inputs without outputs",
			req: &ConfigApplyRequest{
				Inputs: map[string]map[string]any{"i": {"type": "nats"}},
			},
			wantErr: "at least one output is required",
		},
		{
			name: "targets with subscriptions ok",
			req: &ConfigApplyRequest{
				Targets: map[string]*types.TargetConfig{"t": {Name: "t", Address: "127.0.0.1:1"}},
				Subscriptions: map[string]*types.SubscriptionConfig{
					"s": {Name: "s", Paths: []string{"/"}, Mode: "STREAM", StreamMode: "TARGET_DEFINED"},
				},
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApplyRequest(tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeRequest(t *testing.T) {
	req, err := decodeRequest(strings.NewReader(`{"subscriptions":{"s1":{"name":"s1","paths":["/"],"mode":"STREAM","stream-mode":"TARGET_DEFINED"}}}`))
	if err != nil {
		t.Fatalf("decodeRequest: %v", err)
	}
	if len(req.Subscriptions) != 1 || req.Subscriptions["s1"] == nil || req.Subscriptions["s1"].Name != "s1" {
		t.Fatalf("subscriptions: %#v", req.Subscriptions)
	}
}

func TestDecodeRequest_invalidJSON(t *testing.T) {
	_, err := decodeRequest(strings.NewReader(`{`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAssignmentConfig_validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *assignmentConfig
		wantErr string
	}{
		{
			name:    "both empty",
			cfg:     &assignmentConfig{},
			wantErr: "assignments or unassignments is required",
		},
		{
			name: "missing target",
			cfg: &assignmentConfig{
				Assignments: []*assignement{{Member: "m"}},
			},
			wantErr: "target is required",
		},
		{
			name: "missing member",
			cfg: &assignmentConfig{
				Assignments: []*assignement{{Target: "t"}},
			},
			wantErr: "member is required",
		},
		{
			name: "valid assignment",
			cfg: &assignmentConfig{
				Assignments: []*assignement{{Target: "t1", Member: "m1"}},
			},
			wantErr: "",
		},
		{
			name: "valid unassignment only",
			cfg: &assignmentConfig{
				Unassignments: []string{"t1"},
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v want %q", err, tt.wantErr)
			}
		})
	}
}
