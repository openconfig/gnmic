package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
)

// deleteLockLocker records the keys passed to Unlock so a test can assert the lock
// was released, and returns err from Unlock. The embedded interface absorbs the
// methods these tests do not use.
type deleteLockLocker struct {
	lockers.Locker
	unlocked []string
	err      error
}

func (l *deleteLockLocker) Unlock(_ context.Context, key string) error {
	l.unlocked = append(l.unlocked, key)
	return l.err
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

// A target this instance started locking but is not running must still have
// its lock released when it is deleted.
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

// Deleting a target drops its lock cancel func instead of keeping it forever.
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

// A configured target this instance neither runs nor locks has no lock here to
// release. Deleting it succeeds without calling Unlock, which the consul locker
// would answer with an unknown-key error.
func TestDeleteTargetWithoutLocalLockSucceeds(t *testing.T) {
	l := &deleteLockLocker{err: errors.New(`unlock failed: unknown key "gnmic/targets/remote"`)}
	a := newDeleteTestApp(l)

	name := "remote"
	a.Config.Targets[name] = &types.TargetConfig{Name: name}

	if err := a.DeleteTarget(context.Background(), name); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	if a.targetConfigExists(name) {
		t.Fatal("target still in the config after DeleteTarget")
	}
	if len(l.unlocked) != 0 {
		t.Fatalf("unlock called for a target this instance does not lock: %v", l.unlocked)
	}
}

// Once the config entry is deleted, a failed unlock must not turn the delete
// into an error: the API handler would answer 404 for a target it removed.
func TestDeleteTargetUnlockFailureDoesNotFailDelete(t *testing.T) {
	for _, tc := range []struct {
		name    string
		running bool
	}{
		{name: "running", running: true},
		{name: "lock-pending", running: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := &deleteLockLocker{err: errors.New("unlock failed: unknown key")}
			a := newDeleteTestApp(l)
			a.Config.Targets[tc.name] = &types.TargetConfig{Name: tc.name}
			if tc.running {
				a.Targets[tc.name] = target.NewTarget(&types.TargetConfig{Name: tc.name})
			} else {
				_, cancel := context.WithCancel(context.Background())
				a.targetsLockFn[tc.name] = cancel
			}

			if err := a.DeleteTarget(context.Background(), tc.name); err != nil {
				t.Fatalf("DeleteTarget returned %v after deleting the target", err)
			}
			if a.targetConfigExists(tc.name) {
				t.Fatal("target still in the config after DeleteTarget")
			}
			if len(l.unlocked) != 1 {
				t.Fatalf("unlock calls = %v, want exactly 1", l.unlocked)
			}
		})
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
