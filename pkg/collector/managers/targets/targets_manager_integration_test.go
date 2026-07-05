package targets_manager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/collector/managers/testutil"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/prometheus/client_golang/prometheus"
)

func seedSubscription(t *testing.T, st *collstore.Store, name string) {
	t.Helper()
	_, err := st.Config.Set("subscriptions", name, &types.SubscriptionConfig{
		Name:       name,
		Paths:      []string{"/"},
		Mode:       "STREAM",
		StreamMode: "TARGET_DEFINED",
	})
	if err != nil {
		t.Fatalf("seed subscription %s: %v", name, err)
	}
}

func seedTarget(t *testing.T, st *collstore.Store, name, address string) {
	t.Helper()
	_, err := st.Config.Set("targets", name, &types.TargetConfig{
		Name:          name,
		Address:       address,
		Subscriptions: []string{"ifaces"},
	})
	if err != nil {
		t.Fatalf("seed target %s: %v", name, err)
	}
}

func waitForTargetState(t *testing.T, tm *TargetsManager, name string, accept ...string) *collstore.TargetState {
	t.Helper()
	want := make(map[string]struct{}, len(accept))
	for _, s := range accept {
		want[s] = struct{}{}
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ts := tm.GetTargetState(name); ts != nil {
			if _, ok := want[ts.State]; ok {
				return ts
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	got := ""
	if ts := tm.GetTargetState(name); ts != nil {
		got = ts.State
	}
	t.Fatalf("target %q state = %q, want one of %v", name, got, accept)
	return nil
}

func startTargetsManager(t *testing.T, st *collstore.Store) (*TargetsManager, context.CancelFunc, *sync.WaitGroup) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	pipe := make(chan *pipeline.Msg, 16)
	tm := NewTargetsManager(ctx, st, pipe, prometheus.NewRegistry())

	var wg sync.WaitGroup
	if err := tm.Start(testutil.NewFakeLocker(), &wg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
	return tm, cancel, &wg
}

func TestTargetsManager_Start_appliesTargetFromStore(t *testing.T) {
	st := testutil.NewTestStore(t)
	seedSubscription(t, st, "ifaces")
	seedTarget(t, st, "router1", "127.0.0.1:1")

	tm, _, _ := startTargetsManager(t, st)

	waitForTargetState(t, tm, "router1", collstore.StateFailed, collstore.StateStarting)
	if tm.Lookup("router1") == nil {
		t.Fatal("expected managed target after config apply")
	}
}

func TestTargetsManager_Start_skipsUnassignedTargetInCluster(t *testing.T) {
	st := testutil.NewTestStore(t)
	if _, err := st.Config.Set("clustering", "clustering", &config.Clustering{
		ClusterName:  "lab",
		InstanceName: "node1",
	}); err != nil {
		t.Fatalf("seed clustering: %v", err)
	}
	seedSubscription(t, st, "ifaces")
	seedTarget(t, st, "router1", "127.0.0.1:1")

	tm, _, _ := startTargetsManager(t, st)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if tm.Lookup("router1") != nil {
			t.Fatal("unassigned target should not be managed in cluster mode")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTargetsManager_Start_appliesAfterAssignment(t *testing.T) {
	st := testutil.NewTestStore(t)
	if _, err := st.Config.Set("clustering", "clustering", &config.Clustering{
		ClusterName:  "lab",
		InstanceName: "node1",
	}); err != nil {
		t.Fatalf("seed clustering: %v", err)
	}
	seedSubscription(t, st, "ifaces")
	seedTarget(t, st, "router1", "127.0.0.1:1")

	tm, _, _ := startTargetsManager(t, st)

	time.Sleep(100 * time.Millisecond)
	if tm.Lookup("router1") != nil {
		t.Fatal("target should not be managed before assignment")
	}

	if _, err := st.Config.Set("assignments", "router1", map[string]string{
		"target": "router1",
		"member": "node1",
	}); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}

	waitForTargetState(t, tm, "router1", collstore.StateFailed, collstore.StateStarting, collstore.StateRunning)
}

func TestTargetsManager_Start_removesTargetOnDelete(t *testing.T) {
	st := testutil.NewTestStore(t)
	seedSubscription(t, st, "ifaces")
	seedTarget(t, st, "router1", "127.0.0.1:1")

	tm, _, _ := startTargetsManager(t, st)
	waitForTargetState(t, tm, "router1", collstore.StateFailed, collstore.StateStarting)

	if _, _, err := st.Config.Delete("targets", "router1"); err != nil {
		t.Fatalf("delete target: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if tm.Lookup("router1") == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected target removed from manager after config delete")
}

func TestTargetsManager_setTargetState(t *testing.T) {
	st := testutil.NewTestStore(t)
	tm := NewTargetsManager(context.Background(), st, make(chan *pipeline.Msg, 1), prometheus.NewRegistry())

	tm.setTargetState("t1", collstore.StateRunning)
	ts := tm.GetTargetState("t1")
	if ts == nil || ts.State != collstore.StateRunning {
		t.Fatalf("state: %#v", ts)
	}

	states := tm.ListTargetStates()
	if len(states) != 1 {
		t.Fatalf("ListTargetStates len = %d", len(states))
	}
}
