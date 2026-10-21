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
	"github.com/openconfig/gnmic/pkg/lockers"
)

func TestReconcileDeletedTargetsRetriesFailedDeletion(t *testing.T) {
	a := New()
	t.Cleanup(a.Cfn)
	a.Config.Clustering = &config.Clustering{ClusterName: "test", TargetsWatchTimer: time.Second}
	a.Config.Targets["static"] = &types.TargetConfig{Name: "static"}
	stored := map[string]*types.TargetConfig{
		"static": {Name: "static"},
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
	a.reconcileDeletedTargets(context.Background())
	if deletes != 0 {
		t.Fatal("deleted a target before the first successful loader snapshot")
	}
	a.reconcileLoaderSnapshot(context.Background(), map[string]*types.TargetConfig{})
	a.reconcileLoaderSnapshot(context.Background(), map[string]*types.TargetConfig{})
	mu.Lock()
	defer mu.Unlock()
	if deletes != 2 || len(stored) != 1 || stored["static"] == nil {
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
	a.reconcileLoaderSnapshot(context.Background(), map[string]*types.TargetConfig{})
	mu.Lock()
	defer mu.Unlock()
	if len(stored) != 0 {
		t.Fatalf("%d targets remain after reconciliation", len(stored))
	}
}
