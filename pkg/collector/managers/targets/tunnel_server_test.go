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
