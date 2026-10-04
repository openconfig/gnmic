// © 2026 Supranett.
//
// SPDX-License-Identifier: Apache-2.0

package targets_manager

import (
	"context"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
)

// recordingLocker records every Unlock so a test can assert the lock was
// released. The embedded interface satisfies the methods the test does not use.
type recordingLocker struct {
	lockers.Locker
	mu       sync.Mutex
	unlocked []string
}

func (l *recordingLocker) Unlock(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.unlocked = append(l.unlocked, key)
	return nil
}

func (l *recordingLocker) unlockedKeys() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.unlocked...)
}

func newUnlockTestManager(t *testing.T, l lockers.Locker) *TargetsManager {
	t.Helper()
	tm := newTargetsTestManager(t)
	tm.locker = l
	tm.incluster = true
	tm.clustering = &config.Clustering{ClusterName: "test-cluster"}
	return tm
}

// After a restart the target config reloads empty, so a target that was locked
// before the restart is not in tm.targets. remove() only released the lock from
// inside `if mt != nil`, so the ghost lock in the locker backend survived the
// DELETE and nothing could clear it.
func TestRemoveReleasesLockForUnknownTarget(t *testing.T) {
	l := &recordingLocker{}
	tm := newUnlockTestManager(t, l)

	// The target is gone from tm.targets, exactly as after a restart with an
	// empty config, but the locker still holds its lock.
	tm.remove("ghost")

	if got := l.unlockedKeys(); len(got) != 1 {
		t.Fatalf("expected the lock for the unknown target to be released once, got %v", got)
	}
	if got := l.unlockedKeys()[0]; got != tm.targetLockKey("ghost") {
		t.Fatalf("unlocked the wrong key: got %q, want %q", got, tm.targetLockKey("ghost"))
	}
}

// A target that IS known still releases its lock exactly once, through stop().
func TestRemoveReleasesLockForKnownTargetOnce(t *testing.T) {
	l := &recordingLocker{}
	tm := newUnlockTestManager(t, l)
	tm.targets["t1"] = newManagedTarget("t1", &types.TargetConfig{Name: "t1"}, nil)

	tm.remove("t1")

	if got := l.unlockedKeys(); len(got) != 1 {
		t.Fatalf("expected exactly one Unlock for a known target, got %v", got)
	}
}
