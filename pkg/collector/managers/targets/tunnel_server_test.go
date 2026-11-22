package targets_manager

import (
	"log/slog"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
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
		store:  s,
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
		store:  s,
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

func TestAddTargetHandler_preservesOperatorTarget(t *testing.T) {
	s := gomap.NewMemStore[any](store.StoreOptions[any]{})
	defer s.Close()

	if _, err := s.Set("global-flags", "global-flags", config.GlobalFlags{}); err != nil {
		t.Fatalf("seed global-flags: %v", err)
	}
	user := "operator-static"
	if _, err := s.Set("targets", "srl1", &types.TargetConfig{
		Name:     "srl1",
		Address:  "127.0.0.1:1",
		Username: &user,
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	matchUser := "gnmic"
	if _, err := s.Set("tunnel-target-matches", ".*", &config.TunnelTargetMatch{
		ID:   ".*",
		Type: ".*",
		Config: types.TargetConfig{
			Address:  "tunnel:1",
			Username: &matchUser,
		},
	}); err != nil {
		t.Fatalf("seed match: %v", err)
	}

	ts := &tunnelServer{
		store:            s,
		logger:           slog.Default(),
		connectedTargets: map[string]tunnel.Target{},
	}
	if err := ts.addTargetHandler(tunnel.Target{ID: "srl1", Type: "GNMI_GNOI"}); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, ok, err := s.Get("targets", "srl1")
	if err != nil || !ok {
		t.Fatalf("get target: ok=%v err=%v", ok, err)
	}
	cfg, ok := got.(*types.TargetConfig)
	if !ok {
		t.Fatalf("target config: %#v", got)
	}
	if cfg.TunnelTargetType != "" {
		t.Fatalf("tunnel-target-type: %q", cfg.TunnelTargetType)
	}
	if cfg.Username == nil || *cfg.Username != "operator-static" {
		t.Fatalf("username: %v", cfg.Username)
	}
	if cfg.Address != "127.0.0.1:1" {
		t.Fatalf("address: %q", cfg.Address)
	}
}

func TestDeleteTargetHandler_preservesOperatorTarget(t *testing.T) {
	s := gomap.NewMemStore[any](store.StoreOptions[any]{})
	defer s.Close()

	user := "operator-static"
	if _, err := s.Set("targets", "srl1", &types.TargetConfig{
		Name:     "srl1",
		Address:  "127.0.0.1:1",
		Username: &user,
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	ts := &tunnelServer{
		store:  s,
		logger: slog.Default(),
		connectedTargets: map[string]tunnel.Target{
			"srl1": {ID: "srl1", Type: "GNMI_GNOI"},
		},
	}
	if err := ts.deleteTargetHandler(tunnel.Target{ID: "srl1", Type: "GNMI_GNOI"}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, ok, err := s.Get("targets", "srl1")
	if err != nil || !ok {
		t.Fatalf("operator target deleted: ok=%v err=%v", ok, err)
	}
	cfg := got.(*types.TargetConfig)
	if cfg.Username == nil || *cfg.Username != "operator-static" {
		t.Fatalf("username: %v", cfg.Username)
	}
}

func TestReconcileConnectedTargets_preservesOperatorTarget(t *testing.T) {
	s := gomap.NewMemStore[any](store.StoreOptions[any]{})
	defer s.Close()

	if _, err := s.Set("global-flags", "global-flags", config.GlobalFlags{}); err != nil {
		t.Fatalf("seed global-flags: %v", err)
	}
	user := "operator-static"
	if _, err := s.Set("targets", "srl1", &types.TargetConfig{
		Name:     "srl1",
		Address:  "127.0.0.1:1",
		Username: &user,
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	matchUser := "gnmic"
	if _, err := s.Set("tunnel-target-matches", ".*", &config.TunnelTargetMatch{
		ID:   ".*",
		Type: ".*",
		Config: types.TargetConfig{
			Address:  "tunnel:1",
			Username: &matchUser,
		},
	}); err != nil {
		t.Fatalf("seed match: %v", err)
	}

	ts := &tunnelServer{
		store:  s,
		logger: slog.Default(),
		connectedTargets: map[string]tunnel.Target{
			"srl1": {ID: "srl1", Type: "GNMI_GNOI"},
		},
	}
	ts.reconcileConnectedTargets()

	got, ok, err := s.Get("targets", "srl1")
	if err != nil || !ok {
		t.Fatalf("get target: ok=%v err=%v", ok, err)
	}
	cfg := got.(*types.TargetConfig)
	if cfg.TunnelTargetType != "" || cfg.Address != "127.0.0.1:1" {
		t.Fatalf("operator target overwritten: %#v", cfg)
	}
	if cfg.Username == nil || *cfg.Username != "operator-static" {
		t.Fatalf("username: %v", cfg.Username)
	}

	if _, _, err := s.Delete("tunnel-target-matches", ".*"); err != nil {
		t.Fatalf("delete match: %v", err)
	}
	ts.reconcileConnectedTargets()

	if _, ok, err := s.Get("targets", "srl1"); err != nil || !ok {
		t.Fatalf("operator target deleted after match removal: ok=%v err=%v", ok, err)
	}
}
