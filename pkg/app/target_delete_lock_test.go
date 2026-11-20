package app

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
)

// deleteLockLocker records the keys passed to Unlock so a test can assert the lock
// was released. The embedded interface absorbs the methods these tests do not use.
type deleteLockLocker struct {
	lockers.Locker
	unlocked []string
}

func (l *deleteLockLocker) Unlock(_ context.Context, key string) error {
	l.unlocked = append(l.unlocked, key)
	return nil
}

func newDeleteTestApp(l lockers.Locker) *App {
	return &App{
		Config:        config.New(),
		configLock:    new(sync.RWMutex),
		operLock:      new(sync.RWMutex),
		Targets:       make(map[string]*target.Target),
		targetsLockFn: make(map[string]context.CancelFunc),
		locker:        l,
		Logger:        slog.New(slog.DiscardHandler),
	}
}

// A target present in the config but absent from the runtime map is the state a
// collector is in after a restart: it holds locks for targets it has not
// recreated yet. Deleting such a target must still release the lock, otherwise the
// lock is a ghost that no later DELETE against the same config can clear, since
// the fresh runtime map is empty.
func TestDeleteTargetUnlocksWhenTargetNotRunning(t *testing.T) {
	l := new(deleteLockLocker)
	a := newDeleteTestApp(l)

	name := "ghost"
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already-cancelled context stands in for the recorded cancel func
	a.targetsLockFn[name] = cancel
	a.Config.Targets[name] = &types.TargetConfig{Name: name}
	// deliberately not added to a.Targets

	if err := a.DeleteTarget(ctx, name); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}

	want := a.targetLockKey(name)
	if len(l.unlocked) != 1 || l.unlocked[0] != want {
		t.Fatalf("unlock calls = %v, want exactly [%s]", l.unlocked, want)
	}
}

// The assignment is keyed by target name and is reported as live, so deleting the
// target has to drop the entry as well as cancel it.
func TestDeleteTargetRemovesLockAssignment(t *testing.T) {
	l := new(deleteLockLocker)
	a := newDeleteTestApp(l)

	name := "assigned"
	ctx, cancel := context.WithCancel(context.Background())
	a.targetsLockFn[name] = cancel
	a.Config.Targets[name] = &types.TargetConfig{Name: name}

	if err := a.DeleteTarget(ctx, name); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}

	if _, ok := a.targetsLockFn[name]; ok {
		t.Fatal("assignment entry still present after DeleteTarget")
	}
}

// A running target is still removed from the runtime map and still unlocked.
func TestDeleteTargetClosesRunningTarget(t *testing.T) {
	l := new(deleteLockLocker)
	a := newDeleteTestApp(l)

	name := "running"
	a.Config.Targets[name] = &types.TargetConfig{Name: name}
	a.Targets[name] = target.NewTarget(&types.TargetConfig{Name: name})

	if err := a.DeleteTarget(context.Background(), name); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}

	if _, ok := a.Targets[name]; ok {
		t.Fatal("target still in the runtime map after DeleteTarget")
	}
	if len(l.unlocked) != 1 {
		t.Fatalf("unlock calls = %v, want exactly 1", l.unlocked)
	}
}

// Deleting a target that was never there is still an error, so the unlock path
// cannot be reached for a name that does not exist.
func TestDeleteTargetUnknownStillErrors(t *testing.T) {
	l := new(deleteLockLocker)
	a := newDeleteTestApp(l)

	if err := a.DeleteTarget(context.Background(), "absent"); err == nil {
		t.Fatal("expected an error for a target that does not exist")
	}
	if len(l.unlocked) != 0 {
		t.Fatalf("unlock called for an unknown target: %v", l.unlocked)
	}
}
