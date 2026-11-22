package targets_manager

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/collector/managers/testutil"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/prometheus/client_golang/prometheus"
)

// contendedLocker behaves like the consul and k8s lockers: Lock retries
// until the key is free or ctx is done, and Unlock only releases keys this
// member holds.
type contendedLocker struct {
	*testutil.FakeLocker
	self  string
	mu    sync.Mutex
	locks map[string]string
}

func newContendedLocker(self string) *contendedLocker {
	return &contendedLocker{FakeLocker: testutil.NewFakeLocker(), self: self, locks: map[string]string{}}
}

func (l *contendedLocker) hold(key, holder string) {
	l.mu.Lock()
	l.locks[key] = holder
	l.mu.Unlock()
}

func (l *contendedLocker) release(key string) {
	l.mu.Lock()
	delete(l.locks, key)
	l.mu.Unlock()
}

func (l *contendedLocker) holder(key string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.locks[key]
}

func (l *contendedLocker) heldBy(holder string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var keys []string
	for k, v := range l.locks {
		if v == holder {
			keys = append(keys, k)
		}
	}
	return keys
}

func (l *contendedLocker) Lock(ctx context.Context, key string, val []byte) (bool, error) {
	for {
		l.mu.Lock()
		if _, ok := l.locks[key]; !ok {
			l.locks[key] = string(val)
			l.mu.Unlock()
			return true, nil
		}
		l.mu.Unlock()
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (l *contendedLocker) Unlock(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locks[key] != l.self {
		return fmt.Errorf("unknown key %q", key)
	}
	delete(l.locks, key)
	return nil
}

func (l *contendedLocker) List(_ context.Context, prefix string) (map[string]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := map[string]string{}
	for k, v := range l.locks {
		if strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	return out, nil
}

var _ lockers.Locker = (*contendedLocker)(nil)

func lockKey(name string) string { return "gnmic/lab/targets/" + name }

func startClusteredTargetsManager(t *testing.T, l lockers.Locker, reconcileInterval time.Duration) *TargetsManager {
	t.Helper()
	st := testutil.NewTestStore(t)
	if _, err := st.Config.Set("clustering", "clustering", &config.Clustering{ClusterName: "lab", InstanceName: "node1"}); err != nil {
		t.Fatal(err)
	}
	seedSubscription(t, st, "ifaces")
	ctx, cancel := context.WithCancel(context.Background())
	tm := NewTargetsManager(ctx, st, make(chan *pipeline.Msg, 16), prometheus.NewRegistry())
	tm.reconcileInterval = reconcileInterval
	var wg sync.WaitGroup
	if err := tm.Start(l, &wg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); wg.Wait() })
	return tm
}

func seedTimedTarget(t *testing.T, tm *TargetsManager, name string, timeout time.Duration) {
	t.Helper()
	_, err := tm.store.Config.Set("targets", name, &types.TargetConfig{
		Name:          name,
		Address:       "127.0.0.1:1",
		Timeout:       timeout,
		Subscriptions: []string{"ifaces"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assignToSelf(t *testing.T, tm *TargetsManager, name string) {
	t.Helper()
	if _, err := tm.store.Config.Set("assignments", name, map[string]string{"target": name, "member": "node1"}); err != nil {
		t.Fatal(err)
	}
}

func eventually(d time.Duration, f func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if f() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return f()
}

// A target whose lock another member holds must not stop the event loop:
// deleting another target still stops it and releases its lock.
func TestStart_lockHeldElsewhereDoesNotBlockEventLoop(t *testing.T) {
	l := newContendedLocker("node1")
	tm := startClusteredTargetsManager(t, l, time.Hour)

	seedTimedTarget(t, tm, "a", 0)
	assignToSelf(t, tm, "a")
	if !eventually(3*time.Second, func() bool { return l.holder(lockKey("a")) == "node1" }) {
		t.Fatal("a was never locked")
	}
	l.hold(lockKey("b"), "node2")
	seedTimedTarget(t, tm, "b", 300*time.Millisecond)
	assignToSelf(t, tm, "b")
	time.Sleep(100 * time.Millisecond)

	if _, _, err := tm.store.Config.Delete("targets", "a"); err != nil {
		t.Fatal(err)
	}
	if !eventually(3*time.Second, func() bool { return tm.Lookup("a") == nil && l.holder(lockKey("a")) == "" }) {
		t.Fatalf("deleted target a: running=%v, lock holder=%q", tm.Lookup("a") != nil, l.holder(lockKey("a")))
	}
	if got := l.holder(lockKey("b")); got != "node2" {
		t.Fatalf("b lock holder = %q, want node2", got)
	}
}

// Deletes the store's watch channel dropped while the event loop was busy
// are caught up by reconcile: no deleted target keeps running or locked.
func TestReconcile_removesTargetsWhoseDeleteWasDropped(t *testing.T) {
	l := newContendedLocker("node1")
	tm := startClusteredTargetsManager(t, l, 100*time.Millisecond)
	const n = 200
	for i := range n {
		name := fmt.Sprintf("t%03d", i)
		seedTimedTarget(t, tm, name, 0)
		assignToSelf(t, tm, name)
	}
	if !eventually(10*time.Second, func() bool { return len(l.heldBy("node1")) == n }) {
		t.Fatalf("setup: %d/%d targets locked", len(l.heldBy("node1")), n)
	}
	// keep the event loop busy waiting for a lock held by another member
	l.hold(lockKey("blocker"), "node2")
	seedTimedTarget(t, tm, "blocker", 5*time.Second)
	assignToSelf(t, tm, "blocker")
	time.Sleep(200 * time.Millisecond)
	for i := range n {
		if _, _, err := tm.store.Config.Delete("targets", fmt.Sprintf("t%03d", i)); err != nil {
			t.Fatal(err)
		}
	}
	l.release(lockKey("blocker"))

	leaked := func() (running, locked int) {
		for i := range n {
			name := fmt.Sprintf("t%03d", i)
			if tm.Lookup(name) != nil {
				running++
			}
			if l.holder(lockKey(name)) != "" {
				locked++
			}
		}
		return running, locked
	}
	if !eventually(10*time.Second, func() bool { r, k := leaked(); return r == 0 && k == 0 }) {
		r, k := leaked()
		t.Fatalf("%d/%d deleted targets still running, %d still locked", r, n, k)
	}
}

// remove must not wait out the lock timeout of a start that holds mt.Lock
// while it waits for a lock held by another member.
func TestRemove_abortsInFlightLockWait(t *testing.T) {
	l := newContendedLocker("node1")
	l.hold(lockKey("a"), "node2")
	tm := newTargetsTestManager(t)
	tm.locker = l
	tm.incluster = true
	tm.clustering = &config.Clustering{ClusterName: "lab", InstanceName: "node1"}

	applied := make(chan struct{})
	go func() {
		defer close(applied)
		tm.apply("a", &types.TargetConfig{Name: "a", Address: "127.0.0.1:1", Timeout: time.Minute})
	}()
	waiting := func() bool {
		mt := tm.Lookup("a")
		if mt == nil {
			return false
		}
		mt.mu.Lock()
		defer mt.mu.Unlock()
		return mt.lockCancel != nil
	}
	if !eventually(3*time.Second, waiting) {
		t.Fatal("start never waited for the lock")
	}

	removed := make(chan struct{})
	go func() {
		tm.remove("a")
		close(removed)
	}()
	select {
	case <-removed:
	case <-time.After(3 * time.Second):
		t.Fatal("remove blocked behind the lock wait")
	}
	<-applied
	if tm.Lookup("a") != nil {
		t.Fatal("a still running after remove")
	}
	if got := l.holder(lockKey("a")); got != "node2" {
		t.Fatalf("lock holder = %q, want node2", got)
	}
}

// reconcile resyncs this member's assignments from the store, removes
// running targets that are no longer configured or assigned, and starts
// assigned targets that are not running.
func TestReconcile_syncsRunningTargetsWithConfigAndAssignments(t *testing.T) {
	tm := newTargetsTestManager(t)
	l := testutil.NewFakeLocker()
	tm.locker = l
	tm.incluster = true
	tm.clustering = &config.Clustering{ClusterName: "lab", InstanceName: "node1"}

	for _, name := range []string{"missed", "unassigned"} {
		seedTarget(t, tm.store, name, "127.0.0.1:1")
	}
	assignToSelf(t, tm, "missed")
	tm.assignments["unassigned"] = struct{}{} // its unassignment event was dropped
	for _, name := range []string{"deleted", "unassigned"} {
		tm.targets[name] = newManagedTarget(name, &types.TargetConfig{Name: name, Address: "127.0.0.1:1"}, nil)
		l.SetLock(lockKey(name), "node1")
	}

	tm.reconcile()

	if tm.Lookup("missed") == nil {
		t.Error("assigned target missed is not running")
	}
	for _, name := range []string{"deleted", "unassigned"} {
		if tm.Lookup(name) != nil {
			t.Errorf("target %s still running", name)
		}
		if held, _ := l.IsLocked(context.Background(), lockKey(name)); held {
			t.Errorf("target %s still locked", name)
		}
	}
	if tm.amIAssigned("unassigned") {
		t.Error("assignments not resynced from the store")
	}
	tm.remove("missed")
}
