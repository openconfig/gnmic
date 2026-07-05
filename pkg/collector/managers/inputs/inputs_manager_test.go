package inputs_manager

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/inputs"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

type mockInput struct {
	name        string
	startCount  int
	updateCount int
	closed      bool
}

func (m *mockInput) Start(_ context.Context, name string, _ map[string]any, _ ...inputs.Option) error {
	m.name = name
	m.startCount++
	return nil
}

func (m *mockInput) Validate(map[string]any) error { return nil }

func (m *mockInput) Update(map[string]any) error {
	m.updateCount++
	return nil
}

func (m *mockInput) UpdateProcessor(string, map[string]any) error { return nil }

func (m *mockInput) Close() error {
	m.closed = true
	return nil
}

func newInputsTestManager(t *testing.T) (*InputsManager, *collstore.Store, context.CancelFunc) {
	t.Helper()
	cfgStore := gomap.NewMemStore[any](store.StoreOptions[any]{})
	st := collstore.NewStore(cfgStore)
	t.Cleanup(func() { _ = cfgStore.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	pipe := make(chan *pipeline.Msg, 8)
	mgr := NewInputsManager(ctx, st, pipe)
	mgr.logger = slog.Default()
	mgr.inputFactories = map[string]inputs.Initializer{
		"mock": func() inputs.Input { return &mockInput{} },
	}
	return mgr, st, cancel
}

func waitForInputState(t *testing.T, mgr *InputsManager, name, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if st := mgr.GetInputState(name); st != nil && st.State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	got := ""
	if st := mgr.GetInputState(name); st != nil {
		got = st.State
	}
	t.Fatalf("input %q state = %q, want %q", name, got, want)
}

func TestInputsManager_createAndDeleteViaStore(t *testing.T) {
	mgr, st, cancel := newInputsTestManager(t)
	defer cancel()

	var wg sync.WaitGroup
	if err := mgr.Start(&wg); err != nil {
		t.Fatalf("Start: %v", err)
	}

	cfg := map[string]any{"type": "mock"}
	if _, err := st.Config.Set("inputs", "in1", cfg); err != nil {
		t.Fatalf("set input: %v", err)
	}
	waitForInputState(t, mgr, "in1", collstore.StateRunning)

	mgr.mu.RLock()
	mi, ok := mgr.inputs["in1"]
	mgr.mu.RUnlock()
	if !ok {
		t.Fatal("input not registered")
	}
	mi.RLock()
	impl := mi.Impl.(*mockInput)
	mi.RUnlock()
	if impl.startCount != 1 {
		t.Fatalf("startCount = %d, want 1", impl.startCount)
	}

	if _, _, err := st.Config.Delete("inputs", "in1"); err != nil {
		t.Fatalf("delete input config: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mgr.mu.RLock()
		_, ok := mgr.inputs["in1"]
		mgr.mu.RUnlock()
		if !ok && impl.closed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !impl.closed {
		t.Fatal("expected input closed")
	}
	mgr.mu.RLock()
	_, stillThere := mgr.inputs["in1"]
	mgr.mu.RUnlock()
	if stillThere {
		t.Fatal("expected input removed from manager")
	}
}

func TestInputsManager_updateBeforeCreateDoesNotDeadlock(t *testing.T) {
	mgr, _, cancel := newInputsTestManager(t)
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.updateInput("in1", map[string]any{"type": "mock"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("updateInput blocked (deadlock?)")
	}

	waitForInputState(t, mgr, "in1", collstore.StateRunning)
}

func TestInputsManager_updateExistingInput(t *testing.T) {
	mgr, _, cancel := newInputsTestManager(t)
	defer cancel()

	mgr.createInput("in1", map[string]any{"type": "mock"})
	waitForInputState(t, mgr, "in1", collstore.StateRunning)

	mgr.updateInput("in1", map[string]any{"type": "mock", "topic": "telemetry"})
	mgr.mu.RLock()
	impl := mgr.inputs["in1"].Impl.(*mockInput)
	mgr.mu.RUnlock()
	if impl.updateCount != 1 {
		t.Fatalf("updateCount = %d, want 1", impl.updateCount)
	}
}

func TestInputsManager_processorInUse(t *testing.T) {
	mgr, _, cancel := newInputsTestManager(t)
	defer cancel()

	mgr.createInput("in1", map[string]any{
		"type":             "mock",
		"event-processors": []string{"proc1"},
	})
	if !mgr.ProcessorInUse("proc1") {
		t.Fatal("expected proc1 in use")
	}
}
