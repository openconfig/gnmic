// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"strings"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
)

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
