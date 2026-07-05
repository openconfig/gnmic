package targets_manager

import (
	"log/slog"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func newTargetsTestManager(t *testing.T) *TargetsManager {
	t.Helper()
	cfgStore := gomap.NewMemStore[any](store.StoreOptions[any]{})
	st := collstore.NewStore(cfgStore)
	t.Cleanup(func() { _ = cfgStore.Close() })

	pipe := make(chan *pipeline.Msg, 8)
	tm := NewTargetsManager(t.Context(), st, pipe, prometheus.NewRegistry())
	tm.logger = slog.Default()
	return tm
}

func TestCompareOutputs_doesNotMutateConfig(t *testing.T) {
	cfgStore := gomap.NewMemStore[any](store.StoreOptions[any]{})
	defer cfgStore.Close()
	st := collstore.NewStore(cfgStore)

	if _, err := cfgStore.Set("outputs", "prom", map[string]any{"type": "prometheus"}); err != nil {
		t.Fatalf("seed prom output: %v", err)
	}
	if _, err := cfgStore.Set("outputs", "kafka", map[string]any{"type": "kafka"}); err != nil {
		t.Fatalf("seed kafka output: %v", err)
	}

	tm := &TargetsManager{
		store:  st,
		logger: slog.Default(),
	}

	old := &types.TargetConfig{Name: "t1"}
	newCfg := &types.TargetConfig{Name: "t1", Outputs: []string{"prom"}}

	added, removed := tm.compareOutputs(old, newCfg)
	if len(old.Outputs) != 0 {
		t.Fatalf("compareOutputs mutated old.Outputs: %#v", old.Outputs)
	}
	if len(newCfg.Outputs) != 1 || newCfg.Outputs[0] != "prom" {
		t.Fatalf("compareOutputs mutated new.Outputs: %#v", newCfg.Outputs)
	}
	if len(added) != 0 {
		t.Fatalf("added = %#v, want none", added)
	}
	if len(removed) != 1 || removed[0] != "kafka" {
		t.Fatalf("removed = %#v, want [kafka]", removed)
	}
}

func TestCompareSubscriptions_emptyMeansAll(t *testing.T) {
	cfgStore := gomap.NewMemStore[any](store.StoreOptions[any]{})
	defer cfgStore.Close()
	st := collstore.NewStore(cfgStore)

	for _, name := range []string{"sub-a", "sub-b"} {
		if _, err := cfgStore.Set("subscriptions", name, &types.SubscriptionConfig{Name: name}); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	tm := &TargetsManager{store: st, logger: slog.Default()}

	added, removed := tm.compareSubscriptions(nil, []string{"sub-a"})
	if len(added) != 0 {
		t.Fatalf("added = %#v, want none", added)
	}
	if len(removed) != 1 || removed[0] != "sub-b" {
		t.Fatalf("removed = %#v, want [sub-b]", removed)
	}
}

func TestShouldReconnect(t *testing.T) {
	user := "admin"
	base := &types.TargetConfig{Name: "t1", Address: "10.0.0.1:57400", Username: &user}

	if !shouldReconnect(nil, base) {
		t.Fatal("nil -> config should reconnect")
	}
	if shouldReconnect(base, base.DeepCopy()) {
		t.Fatal("equal configs should not reconnect")
	}

	changed := base.DeepCopy()
	changed.Address = "10.0.0.2:57400"
	if !shouldReconnect(base, changed) {
		t.Fatal("address change should reconnect")
	}

	unchanged := base.DeepCopy()
	unchanged.Subscriptions = []string{"ifaces"}
	if shouldReconnect(base, unchanged) {
		t.Fatal("subscription-only change should not reconnect")
	}
}

func TestAmIAssigned_standaloneAndCluster(t *testing.T) {
	tm := newTargetsTestManager(t)

	if !tm.amIAssigned("any-target") {
		t.Fatal("standalone mode should treat all targets as assigned")
	}

	tm.incluster = true
	if tm.amIAssigned("t1") {
		t.Fatal("cluster mode without assignment should be false")
	}

	tm.mas.Lock()
	tm.assignments["t1"] = struct{}{}
	tm.mas.Unlock()
	if !tm.amIAssigned("t1") {
		t.Fatal("expected target to be assigned")
	}

	delete(tm.assignments, "t1")
	if tm.amIAssigned("t1") {
		t.Fatal("expected target unassigned after map delete")
	}
}

func TestTargetLockKey(t *testing.T) {
	tm := newTargetsTestManager(t)
	tm.clustering = &config.Clustering{ClusterName: "lab"}

	if got := tm.targetLockKey("router1"); got != "gnmic/lab/targets/router1" {
		t.Fatalf("targetLockKey = %q", got)
	}
}

func TestTargetConnectionStateFromStr(t *testing.T) {
	tests := []struct {
		in   string
		want targetConnectionState
	}{
		{targetConnectionStateReadyStr, targetConnectionStateReady},
		{targetConnectionStateConnectingStr, targetConnectionStateConnecting},
		{"bogus", targetConnectionStateUnknown},
	}
	for _, tt := range tests {
		if got := targetConnectionStateFromStr(tt.in); got != tt.want {
			t.Fatalf("%q: got %v want %v", tt.in, got, tt.want)
		}
	}
}

func TestHashConnSpec_ignoresSubscriptions(t *testing.T) {
	user := "admin"
	a := &types.TargetConfig{Name: "t1", Address: "10.0.0.1:57400", Username: &user}
	b := a.DeepCopy()
	b.Subscriptions = []string{"ifaces"}

	ha, err := hashConnSpec(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := hashConnSpec(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatal("subscription changes should not affect connection hash")
	}
}

func TestSetIntendedState_requiresManagedTarget(t *testing.T) {
	tm := newTargetsTestManager(t)
	if tm.SetIntendedState("missing", collstore.IntendedStateEnabled) {
		t.Fatal("expected false for unknown target")
	}
}

func TestManagedTarget_lastError(t *testing.T) {
	cfg := &types.TargetConfig{Name: "t1", Address: "10.0.0.1:57400"}
	mt := newManagedTarget("t1", cfg, nil)

	mt.setLastError("boom")
	if got := mt.getLastError(); got != "boom" {
		t.Fatalf("getLastError = %q", got)
	}
	mt.clearLastError()
	if got := mt.getLastError(); got != "" {
		t.Fatalf("getLastError after clear = %q", got)
	}
}

func TestKeys_helper(t *testing.T) {
	got := keys(map[string]int{"b": 1, "a": 2})
	if len(got) != 2 {
		t.Fatalf("keys len = %d", len(got))
	}
}
