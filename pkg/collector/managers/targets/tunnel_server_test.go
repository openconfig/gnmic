package targets_manager

import (
	"log/slog"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
	"github.com/openconfig/grpctunnel/tunnel"
)

// getTunnelTargetMatch must return a non-nil TargetConfig when a connecting
// tunnel target's ID and Type match a configured TunnelTargetMatch rule.
//
// Regression test for a bug where the store.List filter closure in
// getTunnelTargetMatch never returned true on a successful match, so every
// tunnel target registration was rejected with "target ignored, not
// matching any rule" regardless of any configured tunnel-target-matches.
func TestGetTunnelTargetMatch(t *testing.T) {
	s := gomap.NewMemStore[any](store.StoreOptions[any]{})
	defer s.Close()

	if _, err := s.Set("global-flags", "global-flags", config.GlobalFlags{}); err != nil {
		t.Fatalf("failed to seed global-flags: %v", err)
	}

	match := &config.TunnelTargetMatch{
		ID:     "^nbg1-dc8-vc$",
		Type:   "GNMI_GNOI",
		Config: types.TargetConfig{},
	}
	if _, err := s.Set("tunnel-target-matches", "tunnel-policy", match); err != nil {
		t.Fatalf("failed to seed tunnel-target-matches: %v", err)
	}

	ts := &tunnelServer{
		store:  s,
		logger: slog.Default(),
	}

	tc := ts.getTunnelTargetMatch(tunnel.Target{ID: "nbg1-dc8-vc", Type: "GNMI_GNOI"})
	if tc == nil {
		t.Fatal("getTunnelTargetMatch returned nil for a target that matches a configured rule")
	}
	if tc.Name != "nbg1-dc8-vc" {
		t.Errorf("expected target config name %q, got %q", "nbg1-dc8-vc", tc.Name)
	}

	if tc := ts.getTunnelTargetMatch(tunnel.Target{ID: "some-other-target", Type: "GNMI_GNOI"}); tc != nil {
		t.Errorf("expected no match for a non-matching target ID, got %+v", tc)
	}
}

func TestReconcileConnectedTargets_deletesUnmatchedTarget(t *testing.T) {
	s := gomap.NewMemStore[any](store.StoreOptions[any]{})
	defer s.Close()

	if _, err := s.Set("global-flags", "global-flags", config.GlobalFlags{}); err != nil {
		t.Fatalf("seed global-flags: %v", err)
	}

	match := &config.TunnelTargetMatch{
		ID:   "srl1",
		Type: "GNMI_GNOI",
		Config: types.TargetConfig{
			Subscriptions: []string{"ifaces"},
		},
	}
	if _, err := s.Set("tunnel-target-matches", "policy", match); err != nil {
		t.Fatalf("seed match: %v", err)
	}

	tc := &types.TargetConfig{
		Name:             "srl1",
		TunnelTargetType: "GNMI_GNOI",
		Subscriptions:    []string{"ifaces"},
	}
	if _, err := s.Set("targets", "srl1", tc); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	ts := &tunnelServer{
		store: s,
		logger: slog.Default(),
		connectedTargets: map[string]tunnel.Target{
			"srl1": {ID: "srl1", Type: "GNMI_GNOI"},
		},
	}

	if _, _, err := s.Delete("tunnel-target-matches", "policy"); err != nil {
		t.Fatalf("delete match: %v", err)
	}
	ts.reconcileConnectedTargets()

	if _, ok, err := s.Get("targets", "srl1"); err != nil {
		t.Fatalf("get target: %v", err)
	} else if ok {
		t.Fatal("expected tunnel target config deleted after match rule removed")
	}
}

func TestReconcileConnectedTargets_upsertsMatchedTarget(t *testing.T) {
	s := gomap.NewMemStore[any](store.StoreOptions[any]{})
	defer s.Close()

	if _, err := s.Set("global-flags", "global-flags", config.GlobalFlags{}); err != nil {
		t.Fatalf("seed global-flags: %v", err)
	}

	match := &config.TunnelTargetMatch{
		ID:   "srl1",
		Type: "GNMI_GNOI",
		Config: types.TargetConfig{
			Subscriptions: []string{"ifaces"},
		},
	}
	if _, err := s.Set("tunnel-target-matches", "policy", match); err != nil {
		t.Fatalf("seed match: %v", err)
	}

	ts := &tunnelServer{
		store: s,
		logger: slog.Default(),
		connectedTargets: map[string]tunnel.Target{
			"srl1": {ID: "srl1", Type: "GNMI_GNOI"},
		},
	}
	ts.reconcileConnectedTargets()

	got, ok, err := s.Get("targets", "srl1")
	if err != nil {
		t.Fatalf("get target: %v", err)
	}
	if !ok {
		t.Fatal("expected target config upserted")
	}
	cfg, ok := got.(*types.TargetConfig)
	if !ok || cfg.TunnelTargetType != "GNMI_GNOI" {
		t.Fatalf("target config: %#v", got)
	}
}
