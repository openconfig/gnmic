// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/stretchr/testify/require"
)

func TestTargetLockLossCancelsAllSubscriptionsBeforeBlockedRelease(t *testing.T) {
	a := New()
	a.Config.Clustering = &config.Clustering{ClusterName: "test", InstanceName: "pod"}
	a.Config.LocalFlags.SubscribeLockRetry = time.Millisecond
	a.targetsChan = make(chan *target.Target, 2)
	locker := &blockedReleaseLocker{sessions: make(chan lockLossSession, 2), releases: make(chan struct{}, 2)}
	a.locker = locker
	finished := make(chan struct{}, 2)
	t.Cleanup(func() {
		a.Cfn()
		for range 2 {
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Error("target subscription worker did not stop")
			}
		}
	})
	for _, name := range []string{"first", "second"} {
		go func() {
			a.TargetSubscribeStream(a.ctx, &types.TargetConfig{Name: name})
			finished <- struct{}{}
		}()
	}
	sessions := make([]lockLossSession, 0, 2)
	for range 2 {
		select {
		case session := <-locker.sessions:
			sessions = append(sessions, session)
		case <-time.After(time.Second):
			t.Fatal("subscription worker did not acquire ownership")
		}
	}
	for _, session := range sessions {
		session.errs <- errors.New("lease renewal failed")
	}
	for _, session := range sessions {
		select {
		case <-session.ctx.Done():
		case <-time.After(time.Second):
			t.Fatal("subscription context remains active while lock release is blocked")
		}
	}
	for range 2 {
		select {
		case <-locker.releases:
		case <-time.After(time.Second):
			t.Fatal("one target's release blocks another target's cleanup")
		}
	}
	require.Empty(t, a.targetsSnapshot(), "release I/O must not hold the operational mutex")
}

type lockLossSession struct {
	ctx  context.Context
	errs chan error
}

type blockedReleaseLocker struct {
	lockers.Locker
	sessions chan lockLossSession
	releases chan struct{}
}

func (l *blockedReleaseLocker) Lock(context.Context, string, []byte) (bool, error) {
	return true, nil
}

func (l *blockedReleaseLocker) KeepLock(ctx context.Context, _ string) (chan struct{}, chan error) {
	errs := make(chan error, 1)
	l.sessions <- lockLossSession{ctx: ctx, errs: errs}
	return make(chan struct{}), errs
}

func (l *blockedReleaseLocker) Unlock(ctx context.Context, _ string) error {
	l.releases <- struct{}{}
	<-ctx.Done()
	return ctx.Err()
}
