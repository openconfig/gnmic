package targets_manager

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/config"
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

func TestStopTargetSubscription_waitsForReader(t *testing.T) {
	tm := newTargetsTestManager(t)
	mt := newManagedTarget("t1", &types.TargetConfig{Name: "t1", Address: "10.0.0.1:57400"}, nil)

	sctx, cfn := context.WithCancel(t.Context())
	done := make(chan struct{})
	mt.mu.Lock()
	mt.readersCfn["sub1"] = cfn
	mt.readersDone["sub1"] = done
	mt.mu.Unlock()

	go func() {
		<-sctx.Done()
		// The reader also refreshes target state on exit; this must not
		// deadlock with stopTargetSubscription.
		tm.setTargetState("t1", collstore.StateRunning)
		time.Sleep(80 * time.Millisecond)
		close(done)
	}()

	started := time.Now()
	tm.stopTargetSubscription(mt, "sub1")
	if elapsed := time.Since(started); elapsed < 80*time.Millisecond {
		t.Fatalf("stopTargetSubscription returned after %s, want to wait for reader", elapsed)
	}
	select {
	case <-done:
	default:
		t.Fatal("reader still running after stopTargetSubscription")
	}
}

func TestStopTargetSubscription_noReader(t *testing.T) {
	tm := newTargetsTestManager(t)
	mt := newManagedTarget("t1", &types.TargetConfig{Name: "t1", Address: "10.0.0.1:57400"}, nil)
	tm.stopTargetSubscription(mt, "missing")
}

func TestApplySubscription_doesNotHoldWriteLockWhileWaiting(t *testing.T) {
	tm := newTargetsTestManager(t)
	mt := newManagedTarget("t1", &types.TargetConfig{Name: "t1", Address: "10.0.0.1:57400", Subscriptions: []string{"sub1"}}, nil)
	tm.mu.Lock()
	tm.targets["t1"] = mt
	tm.mu.Unlock()

	sctx, cfn := context.WithCancel(t.Context())
	done := make(chan struct{})
	mt.mu.Lock()
	mt.readersCfn["sub1"] = cfn
	mt.readersDone["sub1"] = done
	mt.mu.Unlock()

	go func() {
		<-sctx.Done()
		tm.setTargetState("t1", collstore.StateRunning)
		close(done)
	}()

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		// CreateSubscribeRequest fails (no paths), so we never reach SubscribeChan.
		// The important part is that waiting for the old reader does not deadlock
		// with setTargetState (which takes tm.mu.RLock).
		tm.applySubscription("sub1", types.SubscriptionConfig{Name: "sub1"})
	}()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("applySubscription deadlocked waiting for the old reader")
	}
}
