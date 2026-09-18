package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/loaders"
	"github.com/openconfig/gnmic/pkg/lockers"
)

// oneShotLoader emits a single target and then keeps its channel open, so
// startLoader stays in its receive loop instead of re-initializing.
type oneShotLoader struct {
	ch chan *loaders.TargetOperation
	tc *types.TargetConfig
}

func (l *oneShotLoader) Init(context.Context, map[string]interface{}, *slog.Logger, ...loaders.Option) error {
	return nil
}

func (l *oneShotLoader) RunOnce(context.Context) (map[string]*types.TargetConfig, error) {
	return nil, nil
}

func (l *oneShotLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	go func() {
		select {
		case l.ch <- &loaders.TargetOperation{Add: map[string]*types.TargetConfig{l.tc.Name: l.tc}}:
		case <-ctx.Done():
		}
	}()
	return l.ch
}

func (l *oneShotLoader) RegisterMetrics(*prometheus.Registry)                {}
func (l *oneShotLoader) WithActions(map[string]map[string]interface{})       {}
func (l *oneShotLoader) WithTargetsDefaults(func(*types.TargetConfig) error) {}

// recordingLocker reports nothing locked until the assignment call marks it.
type recordingLocker struct {
	lockers.Locker
	mu     sync.Mutex
	locked map[string]string
}

func (l *recordingLocker) IsLocked(context.Context, string) (bool, error) { return false, nil }

func (l *recordingLocker) List(_ context.Context, prefix string) (map[string]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]string, len(l.locked))
	for k, v := range l.locked {
		if strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	return out, nil
}

func (l *recordingLocker) set(key, val string) {
	l.mu.Lock()
	l.locked[key] = val
	l.mu.Unlock()
}

// The leader can select itself when dispatching a loaded target. The config POST
// it sends to its own API server calls AddTargetConfig, which takes configLock, so
// the loader must not still be holding that lock while it dispatches. It used to
// survive this only because AddTargetConfig checked for an existing target before
// locking and the loader had already inserted it; 89299c15 moved the lock above
// that check and turned the self-assignment into a deadlock.
func TestLoaderDispatchDoesNotDeadlockOnSelfAssignment(t *testing.T) {
	const (
		cluster  = "test"
		instance = "inst-0"
	)

	a := New()
	t.Cleanup(a.Cfn)
	a.Config.Clustering = &config.Clustering{
		ClusterName:             cluster,
		InstanceName:            instance,
		TargetsWatchTimer:       time.Second,
		TargetAssignmentTimeout: 10 * time.Second,
	}
	a.isLeader = true

	locker := &recordingLocker{locked: make(map[string]string)}
	a.locker = locker

	configPostReceived := make(chan struct{}, 1)
	assigned := make(chan struct{}, 1)
	notify := func(ch chan struct{}) {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	// Mirrors handleConfigTargetsPost and handleTargetsPost.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/config/targets":
			notify(configPostReceived)
			tc := new(types.TargetConfig)
			if err := json.NewDecoder(r.Body).Decode(tc); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			a.AddTargetConfig(tc)
		case strings.HasPrefix(r.URL.Path, "/api/v1/targets/"):
			name := strings.TrimPrefix(r.URL.Path, "/api/v1/targets/")
			locker.set("gnmic/"+cluster+"/targets/"+name, instance)
			notify(assigned)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	id := instance + "-api"
	a.apiServices[id] = &lockers.Service{
		ID:      id,
		Address: strings.TrimPrefix(srv.URL, "http://"),
		Tags:    []string{"cluster-name=" + cluster, "instance-name=" + instance},
	}

	const loaderType = "selfassignloadertest"
	loaders.Register(loaderType, func() loaders.TargetLoader {
		return &oneShotLoader{
			ch: make(chan *loaders.TargetOperation),
			tc: &types.TargetConfig{Name: "device-1", Address: "10.0.0.1:57400"},
		}
	})
	a.Config.Loader = map[string]interface{}{"type": loaderType}

	go a.startLoader(a.ctx)

	// startLoader polls a 1s ticker for leadership before it initializes.
	select {
	case <-configPostReceived:
	case <-time.After(15 * time.Second):
		t.Fatal("no target config was POSTed to the selected instance; the loader never dispatched")
	}
	// The assignment POST is only sent once the config POST returns. A blocked
	// handler stalls that until the 5s HTTP client timeout, so a few seconds of
	// silence here means the deadlock is back.
	select {
	case <-assigned:
	case <-time.After(3 * time.Second):
		t.Fatal("target was never assigned: the config POST handler is blocked on configLock held by the loader across dispatchTarget")
	}
}

// dispatchTarget reselects after a failed assignment and spins until
// selectService stops returning a denied service, so every return path has to
// honour the denied list.
func TestSelectServiceHonoursDeniedList(t *testing.T) {
	newApp := func(services map[string][]string) *App {
		a := New()
		t.Cleanup(a.Cfn)
		a.Config.Clustering = &config.Clustering{ClusterName: "test"}
		a.locker = &recordingLocker{locked: make(map[string]string)}
		for id, tags := range services {
			a.apiServices[id] = &lockers.Service{ID: id, Tags: tags}
		}
		return a
	}

	t.Run("sole registered service is denied", func(t *testing.T) {
		a := newApp(map[string][]string{"inst-0-api": nil})
		_, err := a.selectService(nil, "inst-0-api")
		if !errors.Is(err, errNoMoreSuitableServices) {
			t.Fatalf("selectService() err = %v, want %v: the only registered service was already denied", err, errNoMoreSuitableServices)
		}
	})

	t.Run("sole tag-matching service is denied", func(t *testing.T) {
		a := newApp(map[string][]string{
			"inst-0-api": {"role=wjh"},
			"inst-1-api": {"role=other"},
		})
		_, err := a.selectService([]string{"role=wjh"}, "inst-0-api")
		if !errors.Is(err, errNoMoreSuitableServices) {
			t.Fatalf("selectService() err = %v, want %v: the only tag-matching service was already denied", err, errNoMoreSuitableServices)
		}
	})

	t.Run("falls through to an available service", func(t *testing.T) {
		a := newApp(map[string][]string{"inst-0-api": nil, "inst-1-api": nil})
		svc, err := a.selectService(nil, "inst-0-api")
		if err != nil {
			t.Fatalf("selectService() err = %v, want nil: inst-1 is still selectable", err)
		}
		if svc.ID != "inst-1-api" {
			t.Errorf("selectService() = %q, want inst-1-api: the denied service must not be reselected", svc.ID)
		}
	})
}
