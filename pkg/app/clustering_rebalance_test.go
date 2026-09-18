package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
)

// staleLockLocker reports a target lock that has no matching entry in
// a.Config.Targets. The Consul locker does this whenever a lock key outlives the
// target config it was taken for, which is what the rebalance used to trip over.
type staleLockLocker struct {
	lockers.Locker
	locks map[string]string
}

func (l *staleLockLocker) IsLocked(context.Context, string) (bool, error) { return false, nil }

func (l *staleLockLocker) List(_ context.Context, prefix string) (map[string]string, error) {
	out := make(map[string]string, len(l.locks))
	for k, v := range l.locks {
		if strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	return out, nil
}

// A lock whose target has no config must not abort the rebalance, and must not be
// unassigned. Unassigning it strands the target: nothing holds its lock and there
// is no config left to re-dispatch it from, and dispatchTargetsOnce cannot recover
// it because that loop only ranges over a.Config.Targets.
func TestRebalanceSkipsLockWithNoTargetConfig(t *testing.T) {
	const cluster = "test"

	leader := New()
	t.Cleanup(leader.Cfn)
	leader.Config.Clustering = &config.Clustering{
		ClusterName:             cluster,
		TargetAssignmentTimeout: time.Second,
	}
	// "aaa-stale" sorts first, so getInstanceTargets returns it at index 0 and the
	// rebalance reaches it before any target that does have a config.
	leader.locker = &staleLockLocker{locks: map[string]string{
		"gnmic/" + cluster + "/targets/aaa-stale": "collector-a",
		"gnmic/" + cluster + "/targets/device-1":  "collector-a",
		"gnmic/" + cluster + "/targets/device-2":  "collector-a",
	}}
	// aaa-stale is deliberately absent from the config.
	leader.Config.Targets = map[string]*types.TargetConfig{
		"device-1": {Name: "device-1"},
		"device-2": {Name: "device-2"},
	}

	var mu sync.Mutex
	deleted := make(map[string]int)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mu.Lock()
			deleted[strings.TrimPrefix(r.URL.Path, "/api/v1/targets/")]++
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	})
	srvA := httptest.NewServer(handler)
	t.Cleanup(srvA.Close)
	srvB := httptest.NewServer(handler)
	t.Cleanup(srvB.Close)

	leader.clusteringClient = srvA.Client()
	for instance, endpoint := range map[string]string{"collector-a": srvA.URL, "collector-b": srvB.URL} {
		id := instance + "-api"
		leader.apiServices[id] = &lockers.Service{
			ID:      id,
			Address: strings.TrimPrefix(endpoint, "http://"),
			Tags:    []string{"instance-name=" + instance},
		}
	}

	if err := leader.clusterRebalanceTargets(); err != nil {
		t.Fatalf("clusterRebalanceTargets() = %v, want nil: one lock with no target config must not abort the whole rebalance", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if n := deleted["aaa-stale"]; n != 0 {
		t.Errorf("stale lock was unassigned %d time(s), want 0: a target that cannot be re-dispatched must not be unassigned, or it is stranded", n)
	}
	if len(deleted) == 0 {
		t.Error("no target was unassigned, want one of device-1/device-2 moved: the rebalance must carry on past the skipped lock")
	}
	for name := range deleted {
		if name != "device-1" && name != "device-2" {
			t.Errorf("unassigned unexpected target %q, want only a target that has a config", name)
		}
	}
}
