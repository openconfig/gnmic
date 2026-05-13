// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package snmpoutput

import (
	"strings"
	"testing"
)

func TestSNMP_SetDefaults(t *testing.T) {
	s := &snmpOutput{}
	c := &Config{}
	s.setDefaultsFor(c)
	if c.Port != defaultPort {
		t.Errorf("port=%d", c.Port)
	}
	if c.Community != defaultCommunity {
		t.Errorf("community=%q", c.Community)
	}
	if c.StartDelay < minStartDelay {
		t.Errorf("start-delay=%v", c.StartDelay)
	}
}

func TestSNMP_Validate(t *testing.T) {
	s := &snmpOutput{}
	if err := s.Validate(map[string]any{}); err == nil {
		t.Errorf("expected missing traps error")
	}
	// invalid trap (missing trigger)
	if err := s.Validate(map[string]any{
		"traps": []any{map[string]any{}},
	}); err == nil {
		t.Errorf("expected missing trigger error")
	}
	// invalid trap (missing path)
	if err := s.Validate(map[string]any{
		"traps": []any{map[string]any{
			"trigger": map[string]any{"oid": "."},
		}},
	}); err == nil {
		t.Errorf("expected missing path error")
	}
	// valid minimal trap
	err := s.Validate(map[string]any{
		"traps": []any{map[string]any{
			"trigger": map[string]any{"path": "."},
		}},
	})
	if err != nil && !strings.Contains(err.Error(), "registry") {
		t.Errorf("valid trap rejected: %v", err)
	}
	// decode error
	if err := s.Validate(map[string]any{"port": "x"}); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestSNMP_InitializeTrapsErrors(t *testing.T) {
	s := &snmpOutput{}
	cfg := &Config{Traps: []*trap{{Trigger: nil}}}
	if err := s.initializeTrapsFor(cfg); err == nil {
		t.Errorf("expected nil trigger err")
	}
	cfg = &Config{Traps: []*trap{{Trigger: &binding{}}}}
	if err := s.initializeTrapsFor(cfg); err == nil {
		t.Errorf("expected missing path err")
	}
	cfg = &Config{Traps: []*trap{{Trigger: &binding{Path: "."}}}}
	if err := s.initializeTrapsFor(cfg); err != nil {
		t.Errorf("valid trigger: %v", err)
	}
}
