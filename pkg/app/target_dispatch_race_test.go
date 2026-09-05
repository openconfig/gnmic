package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
)

type assignmentRaceLocker struct {
	lockers.Locker
	cancelOnWait bool
}

func (l *assignmentRaceLocker) IsLocked(context.Context, string) (bool, error) {
	return false, nil
}

func (l *assignmentRaceLocker) List(ctx context.Context, prefix string) (map[string]string, error) {
	if l.cancelOnWait && strings.HasSuffix(prefix, "/device-a") {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return map[string]string{"gnmic/test/targets/device-a": "collector-b"}, nil
}

func TestDispatchAcceptsConcurrentOwnerAndHonorsCancellation(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "concurrent-owner"
		if canceled {
			name = "canceled-wait"
		}
		t.Run(name, func(t *testing.T) {
			leader := New()
			t.Cleanup(leader.Cfn)
			leader.Config.Clustering = &config.Clustering{ClusterName: "test", TargetAssignmentTimeout: time.Millisecond}
			leader.locker = &assignmentRaceLocker{cancelOnWait: canceled}
			var starts, deletes, reconciles atomic.Int32
			selected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodDelete:
					deletes.Add(1)
				case r.URL.Path == "/api/v1/config/targets":
					w.WriteHeader(http.StatusOK)
				default:
					starts.Add(1)
				}
			}))
			t.Cleanup(selected.Close)
			owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/config/targets" {
					t.Errorf("unexpected owner request: %s %s", r.Method, r.URL.Path)
				}
				reconciles.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(owner.Close)
			leader.clusteringClient = selected.Client()
			for instance, endpoint := range map[string]string{"collector-a": selected.URL, "collector-b": owner.URL} {
				id := instance + "-api"
				leader.apiServices[id] = &lockers.Service{ID: id, Address: strings.TrimPrefix(endpoint, "http://"), Tags: []string{"instance-name=" + instance}}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- leader.dispatchTarget(ctx, &types.TargetConfig{Name: "device-a"}) }()
			select {
			case err := <-done:
				if canceled {
					if err != context.DeadlineExceeded {
						t.Fatalf("cancellation error = %v", err)
					}
				} else if err != nil {
					t.Fatalf("dispatch to concurrent owner failed: %v", err)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("target dispatch blocked beyond cancellation")
			}
			if starts.Load() != 1 || deletes.Load() != 0 {
				t.Fatalf("unnecessary redispatch: starts=%d deletes=%d", starts.Load(), deletes.Load())
			}
		})
	}
}
