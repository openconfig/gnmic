// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/loaders"
	"github.com/openconfig/gnmic/pkg/lockers"
)

func TestReconcileDeletedTargetsRetriesFailedDeletion(t *testing.T) {
	a := New()
	t.Cleanup(a.Cfn)
	a.Config.Clustering = &config.Clustering{ClusterName: "test", TargetsWatchTimer: time.Second}
	a.Config.Targets["static"] = &types.TargetConfig{Name: "static"}
	stored := map[string]*types.TargetConfig{
		"static": {Name: "static"},
		"manual": {Name: "manual"},
		"stale":  {Name: "stale"},
	}
	var mu sync.Mutex
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(stored)
		case http.MethodDelete:
			if r.URL.Path != "/api/v1/config/targets/stale" {
				t.Errorf("unexpected deletion: %s", r.URL.Path)
				return
			}
			deletes++
			if deletes == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			delete(stored, "stale")
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(server.Close)
	a.clusteringClient = server.Client()
	a.apiServices["collector-a-api"] = &lockers.Service{ID: "collector-a-api", Address: strings.TrimPrefix(server.URL, "http://")}
	a.reconcileLoaderSnapshot(context.Background(), map[string]*types.TargetConfig{"stale": {Name: "stale"}})
	if deletes != 0 {
		t.Fatal("deleted a target that is still in the loader snapshot")
	}
	a.reconcileLoaderSnapshot(context.Background(), map[string]*types.TargetConfig{})
	a.reconcileLoaderSnapshot(context.Background(), map[string]*types.TargetConfig{})
	mu.Lock()
	defer mu.Unlock()
	if deletes != 2 || len(stored) != 2 || stored["static"] == nil || stored["manual"] == nil {
		t.Fatalf("deletes=%d remaining=%v", deletes, stored)
	}
}

func TestReconcileDeletedRuntimeTargetWithoutConfig(t *testing.T) {
	a := New()
	t.Cleanup(a.Cfn)
	a.Config.Clustering = &config.Clustering{ClusterName: "test", TargetsWatchTimer: time.Second}
	runtime := map[string]json.RawMessage{"stale": json.RawMessage(`{}`)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Path == "/api/v1/config/targets" {
				_, _ = w.Write([]byte(`{}`))
				return
			}
			_ = json.NewEncoder(w).Encode(runtime)
		case http.MethodDelete:
			delete(runtime, strings.TrimPrefix(r.URL.Path, "/api/v1/config/targets/"))
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(server.Close)
	a.clusteringClient = server.Client()
	a.apiServices["collector-a-api"] = &lockers.Service{ID: "collector-a-api", Address: strings.TrimPrefix(server.URL, "http://")}
	a.applyLoaderSnapshot(map[string]*types.TargetConfig{"stale": {Name: "stale"}})
	a.reconcileLoaderSnapshot(context.Background(), map[string]*types.TargetConfig{})
	if len(runtime) != 0 {
		t.Fatalf("runtime targets remain: %v", runtime)
	}
}

func TestLoaderSnapshotWaitsForDispatch(t *testing.T) {
	a := New()
	t.Cleanup(a.Cfn)
	a.Config.Clustering = &config.Clustering{ClusterName: "test", TargetsWatchTimer: time.Second}
	a.applyLoaderSnapshot(map[string]*types.TargetConfig{"stale": {Name: "stale"}})
	a.dispatchLock.Lock()
	a.configLock.Lock()
	delete(a.Config.Targets, "stale")
	a.configLock.Unlock()
	done := make(chan struct{})
	go func() {
		a.reconcileLoaderSnapshot(context.Background(), map[string]*types.TargetConfig{})
		close(done)
	}()
	select {
	case <-done:
		a.dispatchLock.Unlock()
		t.Fatal("snapshot advanced while a dispatch was still running")
	case <-time.After(20 * time.Millisecond):
	}
	a.configLock.Lock()
	a.Config.Targets["stale"] = &types.TargetConfig{Name: "stale"}
	a.configLock.Unlock()
	a.dispatchLock.Unlock()
	<-done
	if a.targetConfigExists("stale") {
		t.Fatal("late dispatched target remains after snapshot")
	}
}

func TestReconcileDeletedTargetsBatch(t *testing.T) {
	a := New()
	t.Cleanup(a.Cfn)
	a.Config.Clustering = &config.Clustering{ClusterName: "test", TargetsWatchTimer: 2 * time.Second}
	stored := make(map[string]*types.TargetConfig, 640)
	for i := range 640 {
		name := fmt.Sprintf("device-%03d", i)
		stored[name] = &types.TargetConfig{Name: name}
	}
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(stored)
		case http.MethodDelete:
			delete(stored, strings.TrimPrefix(r.URL.Path, "/api/v1/config/targets/"))
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(server.Close)
	a.clusteringClient = server.Client()
	a.apiServices["collector-a-api"] = &lockers.Service{ID: "collector-a-api", Address: strings.TrimPrefix(server.URL, "http://")}
	a.applyLoaderSnapshot(stored)
	a.reconcileLoaderSnapshot(context.Background(), map[string]*types.TargetConfig{})
	mu.Lock()
	defer mu.Unlock()
	if len(stored) != 0 {
		t.Fatalf("%d targets remain after reconciliation", len(stored))
	}
}

// scriptedLoader emits its operations in order and keeps its channel open.
type scriptedLoader struct {
	oneShotLoader
	ops []*loaders.TargetOperation
}

func (l *scriptedLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	ch := make(chan *loaders.TargetOperation)
	go func() {
		for _, op := range l.ops {
			select {
			case ch <- op:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// memberLocker reports a target as locked while the member holds its assignment.
type memberLocker struct{ *recordingLocker }

func (l memberLocker) IsLocked(_ context.Context, key string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, locked := l.locked[key]
	return locked, nil
}

func TestLoaderSnapshotDispatchesAddedAndChangedTargets(t *testing.T) {
	const cluster, instance, name = "test", "inst-0", "rack/router"
	key := "gnmic/" + cluster + "/targets/" + name
	a := New()
	t.Cleanup(a.Cfn)
	a.Config.Clustering = &config.Clustering{
		ClusterName:             cluster,
		InstanceName:            instance,
		TargetsWatchTimer:       time.Minute,
		TargetAssignmentTimeout: 5 * time.Second,
	}
	a.isLeader = true
	locker := memberLocker{&recordingLocker{locked: make(map[string]string)}}
	a.locker = locker
	assigned := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config/targets":
			tc := new(types.TargetConfig)
			_ = json.NewDecoder(r.Body).Decode(tc)
			assigned <- tc.Address
		case r.Method == http.MethodPost:
			locker.set(key, instance)
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/api/v1/config/targets/rack%2Frouter":
			locker.mu.Lock()
			delete(locker.locked, key)
			locker.mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	a.clusteringClient = server.Client()
	a.apiServices[instance+"-api"] = &lockers.Service{
		ID:      instance + "-api",
		Address: strings.TrimPrefix(server.URL, "http://"),
		Tags:    []string{"cluster-name=" + cluster, "instance-name=" + instance},
	}
	const firstAddress, changedAddress = "192.0.2.1:57400", "192.0.2.2:57400"
	first := &types.TargetConfig{Name: name, Address: firstAddress}
	changed := &types.TargetConfig{Name: name, Address: changedAddress}
	const loaderType = "snapshotdispatchtest"
	loaders.Register(loaderType, func() loaders.TargetLoader {
		return &scriptedLoader{ops: []*loaders.TargetOperation{
			{Add: map[string]*types.TargetConfig{name: first}, Snapshot: map[string]*types.TargetConfig{name: first}},
			{Del: []string{name}, Add: map[string]*types.TargetConfig{name: changed}, Snapshot: map[string]*types.TargetConfig{name: changed}},
		}}
	})
	a.Config.Loader = map[string]interface{}{"type": loaderType}
	go a.startLoader(a.ctx)

	// The dispatch loop is not running, so each assignment comes from the loader itself.
	for _, want := range []string{firstAddress, changedAddress} {
		select {
		case address := <-assigned:
			if address != want {
				t.Fatalf("assigned %s, want %s", address, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("target with address %s was not dispatched after the loader snapshot", want)
		}
	}
}
