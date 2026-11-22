package targets_manager

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/loaders"
	"github.com/prometheus/client_golang/prometheus"
)

type fakeLoader struct {
	ch chan *loaders.TargetOperation
}

func newFakeLoader() *fakeLoader {
	return &fakeLoader{ch: make(chan *loaders.TargetOperation)}
}

func (f *fakeLoader) Init(context.Context, map[string]any, *slog.Logger, ...loaders.Option) error {
	return nil
}

func (f *fakeLoader) RunOnce(context.Context) (map[string]*types.TargetConfig, error) {
	return nil, nil
}

func (f *fakeLoader) Start(context.Context) chan *loaders.TargetOperation { return f.ch }

func (f *fakeLoader) RegisterMetrics(*prometheus.Registry) {}

func (f *fakeLoader) WithActions(map[string]map[string]any) {}

func (f *fakeLoader) WithTargetsDefaults(func(tc *types.TargetConfig) error) {}

func loaderTarget(name, address string, subs ...string) *types.TargetConfig {
	return &types.TargetConfig{Name: name, Address: address, Subscriptions: subs}
}

// startTestLoader runs startLoader until the test ends. Sends on the returned
// channel are unbuffered, so each send returns only after the previous
// operation was fully applied.
func startTestLoader(t *testing.T, tm *TargetsManager) chan *loaders.TargetOperation {
	t.Helper()
	fl := newFakeLoader()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tm.startLoader(ctx, fl)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return fl.ch
}

func sendOp(t *testing.T, ch chan *loaders.TargetOperation, op *loaders.TargetOperation) {
	t.Helper()
	select {
	case ch <- op:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out sending loader operation")
	}
}

// flush waits until every earlier operation has been applied.
func flush(t *testing.T, ch chan *loaders.TargetOperation) {
	t.Helper()
	sendOp(t, ch, &loaders.TargetOperation{})
}

func storedTarget(t *testing.T, tm *TargetsManager, name string) (*types.TargetConfig, bool) {
	t.Helper()
	v, ok, err := tm.store.Config.Get("targets", name)
	if err != nil {
		t.Fatalf("get target %q: %v", name, err)
	}
	if !ok {
		return nil, false
	}
	tc, isTC := v.(*types.TargetConfig)
	if !isTC {
		t.Fatalf("target %q stored as %T", name, v)
	}
	return tc, true
}

func TestStartLoader_changedTargetIsUpdated(t *testing.T) {
	tm := newTargetsTestManager(t)
	ch := startTestLoader(t, tm)

	before := map[string]*types.TargetConfig{
		"r1": loaderTarget("r1", "127.0.0.1:1", "small"),
	}
	sendOp(t, ch, loaders.Diff(nil, before))

	after := map[string]*types.TargetConfig{
		"r1": loaderTarget("r1", "127.0.0.1:2", "small"),
	}
	op := loaders.Diff(before, after)
	if _, ok := op.Add["r1"]; !ok || len(op.Del) != 1 || op.Del[0] != "r1" {
		t.Fatalf("Diff no longer reports a change as add+del: %+v", op)
	}
	sendOp(t, ch, op)
	flush(t, ch)

	tc, ok := storedTarget(t, tm, "r1")
	if !ok {
		t.Fatal("changed target r1 was deleted from the store")
	}
	if tc.Address != "127.0.0.1:2" {
		t.Fatalf("r1 address = %q, want 127.0.0.1:2", tc.Address)
	}
}

func TestStartLoader_addsAndRemovesTargets(t *testing.T) {
	tm := newTargetsTestManager(t)
	ch := startTestLoader(t, tm)

	before := map[string]*types.TargetConfig{
		"r1": loaderTarget("r1", "127.0.0.1:1"),
		"r2": loaderTarget("r2", "127.0.0.1:2"),
	}
	sendOp(t, ch, loaders.Diff(nil, before))

	after := map[string]*types.TargetConfig{
		"r2": loaderTarget("r2", "127.0.0.1:2"),
		"r3": loaderTarget("r3", "127.0.0.1:3"),
	}
	sendOp(t, ch, loaders.Diff(before, after))
	flush(t, ch)

	if _, ok := storedTarget(t, tm, "r1"); ok {
		t.Fatal("removed target r1 is still in the store")
	}
	for _, name := range []string{"r2", "r3"} {
		if _, ok := storedTarget(t, tm, name); !ok {
			t.Fatalf("target %s is missing from the store", name)
		}
	}
}

func TestStartLoader_storesCopyOfLoaderConfig(t *testing.T) {
	tm := newTargetsTestManager(t)
	ch := startTestLoader(t, tm)

	loaded := loaderTarget("r1", "127.0.0.1:1", "small")
	sendOp(t, ch, &loaders.TargetOperation{
		Add: map[string]*types.TargetConfig{"r1": loaded},
	})
	flush(t, ch)

	// Same in-place edit as PATCH /api/v1/config/targets/{name}/subscriptions.
	_, err := tm.store.Config.SetFn("targets", "r1", func(v any) (any, error) {
		tc := v.(*types.TargetConfig)
		tc.Subscriptions = []string{"other"}
		return tc, nil
	})
	if err != nil {
		t.Fatalf("patch stored target: %v", err)
	}

	if len(loaded.Subscriptions) != 1 || loaded.Subscriptions[0] != "small" {
		t.Fatalf("API edit changed the loader's config: subscriptions = %v", loaded.Subscriptions)
	}
	if op := loaders.Diff(
		map[string]*types.TargetConfig{"r1": loaded},
		map[string]*types.TargetConfig{"r1": loaderTarget("r1", "127.0.0.1:1", "small")},
	); len(op.Add) != 0 || len(op.Del) != 0 {
		t.Fatalf("unchanged source produced a loader operation after an API edit: %+v", op)
	}
}

func TestStartLoader_returnsWhenChannelCloses(t *testing.T) {
	tm := newTargetsTestManager(t)
	fl := newFakeLoader()
	done := make(chan struct{})
	go func() {
		defer close(done)
		tm.startLoader(t.Context(), fl)
	}()
	close(fl.ch)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("startLoader kept running after the loader channel closed")
	}
}
