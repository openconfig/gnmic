// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	coordinationclient "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/utils/ptr"
)

func TestReleaseUsesIndependentTimeouts(t *testing.T) {
	const requestTimeout = 500 * time.Millisecond
	client := fake.NewClientset(testLease("key", "owner", time.Now()))
	budgets := &releaseBudgetLeases{
		LeaseInterface: client.CoordinationV1().Leases("test"),
		getBudget:      make(chan time.Duration, 1),
		deleteBudget:   make(chan time.Duration, 1),
		getDelay:       150 * time.Millisecond,
	}
	k := &k8sLocker{
		Cfg:       &config{Namespace: "test", RetryPeriod: requestTimeout},
		clientset: interceptedClient{Clientset: client, leases: budgets},
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	session := &leaseSession{ctx: ctx, cancel: cancel, identity: "foreign-session", stopped: make(chan struct{})}
	go func() {
		<-ctx.Done()
		time.Sleep(150 * time.Millisecond)
		close(session.stopped)
	}()

	require.NoError(t, k.release(t.Context(), "key", session))
	require.Greater(t, <-budgets.getBudget, 350*time.Millisecond)
	require.Greater(t, <-budgets.deleteBudget, 350*time.Millisecond)
}

func TestStopBoundsConcurrentReleases(t *testing.T) {
	const sessionCount = maxConcurrentReleases * 2
	client := fake.NewClientset()
	entered := make(chan struct{}, sessionCount)
	unblock := make(chan struct{})
	var unblockOnce sync.Once
	releaseWorkers := func() { unblockOnce.Do(func() { close(unblock) }) }
	defer releaseWorkers()
	var active, maximum atomic.Int32
	leases := &concurrentReleaseLeases{
		LeaseInterface: client.CoordinationV1().Leases("test"),
		entered:        entered,
		unblock:        unblock,
		active:         &active,
		maximum:        &maximum,
	}

	sessions := make(map[string]*leaseSession, sessionCount)
	for i := range sessionCount {
		ctx, cancel := context.WithCancelCause(t.Context())
		stopped := make(chan struct{})
		close(stopped)
		sessions[fmt.Sprintf("key-%d", i)] = &leaseSession{ctx: ctx, cancel: cancel, identity: "owner", stopped: stopped}
	}
	k := &k8sLocker{
		Cfg:       &config{Namespace: "test", RetryPeriod: time.Second},
		clientset: interceptedClient{Clientset: client, leases: leases},
		locks:     sessions,
	}
	result := make(chan error, 1)
	go func() { result <- k.Stop() }()
	for started := 0; started < maxConcurrentReleases; started++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatalf("release worker did not start: got %d, want %d", started, maxConcurrentReleases)
		}
	}
	select {
	case <-entered:
		t.Fatal("release concurrency exceeded its limit")
	default:
	}
	require.Equal(t, int32(maxConcurrentReleases), maximum.Load())
	releaseWorkers()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("stop did not wait for every release worker")
	}
	require.Zero(t, active.Load())
}

type releaseBudgetLeases struct {
	coordinationclient.LeaseInterface
	getBudget    chan time.Duration
	deleteBudget chan time.Duration
	getDelay     time.Duration
}

type concurrentReleaseLeases struct {
	coordinationclient.LeaseInterface
	entered chan<- struct{}
	unblock <-chan struct{}
	active  *atomic.Int32
	maximum *atomic.Int32
}

func (l *concurrentReleaseLeases) Get(ctx context.Context, name string, _ metav1.GetOptions) (*coordinationv1.Lease, error) {
	current := l.active.Add(1)
	defer l.active.Add(-1)
	for observed := l.maximum.Load(); current > observed && !l.maximum.CompareAndSwap(observed, current); observed = l.maximum.Load() {
	}
	l.entered <- struct{}{}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.unblock:
		return &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
			Spec:       coordinationv1.LeaseSpec{HolderIdentity: ptr.To("owner")},
		}, nil
	}
}

func (l *concurrentReleaseLeases) Delete(context.Context, string, metav1.DeleteOptions) error {
	return nil
}

func (l *releaseBudgetLeases) Get(ctx context.Context, name string, opts metav1.GetOptions) (*coordinationv1.Lease, error) {
	deadline, _ := ctx.Deadline()
	l.getBudget <- time.Until(deadline)
	timer := time.NewTimer(l.getDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return l.LeaseInterface.Get(ctx, name, opts)
	}
}

func (l *releaseBudgetLeases) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	deadline, _ := ctx.Deadline()
	l.deleteBudget <- time.Until(deadline)
	return l.LeaseInterface.Delete(ctx, name, opts)
}
