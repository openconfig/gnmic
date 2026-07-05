package outputs_manager

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/config"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
	"google.golang.org/protobuf/proto"
)

type mockOutput struct {
	mu          sync.Mutex
	name        string
	clusterName string
	initCount   int
	updateCount int
	writeCount  int
	eventCount  int
	closed      bool
}

func (m *mockOutput) eventCountSnapshot() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.eventCount
}

func (m *mockOutput) Init(_ context.Context, name string, _ map[string]any, opts ...outputs.Option) error {
	o := &outputs.OutputOptions{}
	for _, opt := range opts {
		_ = opt(o)
	}
	m.name = name
	m.clusterName = o.ClusterName
	m.initCount++
	return nil
}

func (m *mockOutput) Validate(map[string]any) error { return nil }

func (m *mockOutput) Update(_ context.Context, _ map[string]any) error {
	m.updateCount++
	return nil
}

func (m *mockOutput) UpdateProcessor(string, map[string]any) error { return nil }

func (m *mockOutput) Write(context.Context, proto.Message, outputs.Meta) {
	m.mu.Lock()
	m.writeCount++
	m.mu.Unlock()
}

func (m *mockOutput) WriteEvent(context.Context, *formatters.EventMsg) {
	m.mu.Lock()
	m.eventCount++
	m.mu.Unlock()
}

func (m *mockOutput) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockOutput) String() string { return m.name }

func newOutputsTestManager(t *testing.T) (*OutputsManager, *collstore.Store, chan *pipeline.Msg, context.CancelFunc) {
	t.Helper()
	cfgStore := gomap.NewMemStore[any](store.StoreOptions[any]{})
	st := collstore.NewStore(cfgStore)
	t.Cleanup(func() { _ = cfgStore.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	pipe := make(chan *pipeline.Msg, 32)
	mgr := NewOutputsManager(ctx, st, pipe, nil)
	mgr.logger = slog.Default()
	mgr.OutputsFactory = map[string]outputs.Initializer{
		"mock": func() outputs.Output { return &mockOutput{} },
	}
	return mgr, st, pipe, cancel
}

func waitForOutputState(t *testing.T, mgr *OutputsManager, name, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := mgr.getOutputStateStr(name); got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("output %q state = %q, want %q", name, mgr.getOutputStateStr(name), want)
}

func TestOutputsManager_createAndDeleteViaStore(t *testing.T) {
	mgr, st, _, cancel := newOutputsTestManager(t)
	defer cancel()

	var wg sync.WaitGroup
	if err := mgr.Start(nil, &wg); err != nil {
		t.Fatalf("Start: %v", err)
	}

	cfg := map[string]any{"type": "mock"}
	if _, err := st.Config.Set("outputs", "out1", cfg); err != nil {
		t.Fatalf("set output: %v", err)
	}
	waitForOutputState(t, mgr, "out1", collstore.StateRunning)

	mgr.mu.RLock()
	mo, ok := mgr.outputs["out1"]
	mgr.mu.RUnlock()
	if !ok {
		t.Fatal("output not registered in manager")
	}
	mo.RLock()
	impl := mo.Impl.(*mockOutput)
	mo.RUnlock()
	if impl.initCount != 1 {
		t.Fatalf("initCount = %d, want 1", impl.initCount)
	}

	if _, _, err := st.Config.Delete("outputs", "out1"); err != nil {
		t.Fatalf("delete output config: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mgr.mu.RLock()
		_, ok := mgr.outputs["out1"]
		mgr.mu.RUnlock()
		if !ok && impl.closed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !impl.closed {
		t.Fatal("expected output to be closed on delete")
	}
	mgr.mu.RLock()
	_, stillThere := mgr.outputs["out1"]
	mgr.mu.RUnlock()
	if stillThere {
		t.Fatal("expected output removed from manager")
	}
}

func TestOutputsManager_updateBeforeCreateDoesNotDeadlock(t *testing.T) {
	mgr, _, _, cancel := newOutputsTestManager(t)
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.updateOutput("out1", map[string]any{"type": "mock"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("updateOutput blocked (deadlock?)")
	}

	waitForOutputState(t, mgr, "out1", collstore.StateRunning)
}

func TestOutputsManager_updateExistingOutput(t *testing.T) {
	mgr, _, _, cancel := newOutputsTestManager(t)
	defer cancel()

	mgr.createOutput("out1", map[string]any{"type": "mock"})
	waitForOutputState(t, mgr, "out1", collstore.StateRunning)

	mgr.updateOutput("out1", map[string]any{"type": "mock", "endpoint": "x"})
	mgr.mu.RLock()
	impl := mgr.outputs["out1"].Impl.(*mockOutput)
	mgr.mu.RUnlock()
	if impl.updateCount != 1 {
		t.Fatalf("updateCount = %d, want 1", impl.updateCount)
	}
}

func TestOutputsManager_clusterNameFromStore(t *testing.T) {
	mgr, st, _, cancel := newOutputsTestManager(t)
	defer cancel()

	if _, err := st.Config.Set("clustering", "clustering", &config.Clustering{
		ClusterName:  "prod",
		InstanceName: "node1",
	}); err != nil {
		t.Fatalf("set clustering: %v", err)
	}

	mgr.createOutput("out1", map[string]any{"type": "mock"})

	mgr.mu.RLock()
	impl := mgr.outputs["out1"].Impl.(*mockOutput)
	mgr.mu.RUnlock()
	if impl.clusterName != "prod" {
		t.Fatalf("clusterName = %q, want prod", impl.clusterName)
	}
}

func TestOutputsManager_getOutputsForTarget(t *testing.T) {
	mgr, _, _, cancel := newOutputsTestManager(t)
	defer cancel()

	mgr.createOutput("a", map[string]any{"type": "mock"})
	mgr.createOutput("b", map[string]any{"type": "mock"})
	waitForOutputState(t, mgr, "a", collstore.StateRunning)
	waitForOutputState(t, mgr, "b", collstore.StateRunning)

	all := mgr.getOutputsForTarget(nil)
	if len(all) != 2 {
		t.Fatalf("all outputs: got %d want 2", len(all))
	}

	specific := mgr.getOutputsForTarget(map[string]struct{}{"a": {}})
	if len(specific) != 1 || specific[0].Name != "a" {
		t.Fatalf("specific outputs: %#v", specific)
	}
}

func TestExtractProcessors(t *testing.T) {
	if got := extractProcessors(map[string]any{}); got != nil {
		t.Fatalf("got %#v", got)
	}
	got := extractProcessors(map[string]any{
		"event-processors": []any{"p1", "p2"},
	})
	if len(got) != 2 || got[0] != "p1" || got[1] != "p2" {
		t.Fatalf("got %#v", got)
	}
}

func TestOutputsManager_processorInUse(t *testing.T) {
	mgr, _, _, cancel := newOutputsTestManager(t)
	defer cancel()

	mgr.createOutput("out1", map[string]any{
		"type":             "mock",
		"event-processors": []any{"proc1"},
	})
	if !mgr.ProcessorInUse("proc1") {
		t.Fatal("expected proc1 to be in use")
	}
	if mgr.ProcessorInUse("missing") {
		t.Fatal("unexpected processor in use")
	}
}

func TestOutputsManager_writeDispatchesToOutputs(t *testing.T) {
	mgr, _, _, cancel := newOutputsTestManager(t)
	defer cancel()

	mgr.createOutput("out-a", map[string]any{"type": "mock"})
	mgr.createOutput("out-b", map[string]any{"type": "mock"})
	waitForOutputState(t, mgr, "out-a", collstore.StateRunning)
	waitForOutputState(t, mgr, "out-b", collstore.StateRunning)

	mgr.write(&pipeline.Msg{
		Meta:    outputs.Meta{"subscription-name": "ifaces"},
		Outputs: map[string]struct{}{"out-a": {}},
		Events:  []*formatters.EventMsg{{Name: "metric", Values: map[string]any{"v": int64(1)}}},
	})

	mgr.mu.RLock()
	a := mgr.outputs["out-a"].Impl.(*mockOutput)
	b := mgr.outputs["out-b"].Impl.(*mockOutput)
	mgr.mu.RUnlock()
	if a.eventCountSnapshot() != 1 {
		t.Fatalf("out-a events = %d, want 1", a.eventCountSnapshot())
	}
	if b.eventCountSnapshot() != 0 {
		t.Fatalf("out-b events = %d, want 0", b.eventCountSnapshot())
	}
}

func waitForMockEventCount(t *testing.T, mgr *OutputsManager, name string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mgr.mu.RLock()
		mo, ok := mgr.outputs[name]
		mgr.mu.RUnlock()
		if !ok {
			t.Fatalf("output %q not found", name)
		}
		if mo.Impl.(*mockOutput).eventCountSnapshot() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	mgr.mu.RLock()
	mo := mgr.outputs[name]
	mgr.mu.RUnlock()
	t.Fatalf("output %q eventCount = %d, want %d", name, mo.Impl.(*mockOutput).eventCountSnapshot(), want)
}

func TestOutputsManager_writeLoopConcurrentMessages(t *testing.T) {
	mgr, _, pipe, cancel := newOutputsTestManager(t)
	defer cancel()

	mgr.createOutput("out1", map[string]any{"type": "mock"})
	waitForOutputState(t, mgr, "out1", collstore.StateRunning)

	var wg sync.WaitGroup
	wg.Add(1)
	go mgr.writeLoop(&wg)

	const n = 20
	for i := 0; i < n; i++ {
		pipe <- &pipeline.Msg{
			Events: []*formatters.EventMsg{{Name: "m", Values: map[string]any{"i": i}}},
		}
	}

	waitForMockEventCount(t, mgr, "out1", n)
	cancel()
	wg.Wait()
}
