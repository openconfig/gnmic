// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	coordinationclient "k8s.io/client-go/kubernetes/typed/coordination/v1"
	clienttesting "k8s.io/client-go/testing"
)

func TestRenewalDeadlineStopsOwnershipAndAllowsReacquisition(t *testing.T) {
	client := fake.NewClientset()
	faults := &blockingLeases{LeaseInterface: client.CoordinationV1().Leases("test"), entered: make(chan time.Time, 1)}
	k := testLocker(t, interceptedClient{Clientset: client, leases: faults})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ok, err := k.Lock(ctx, "key", []byte("pod"))
	require.NoError(t, err)
	require.True(t, ok)
	done, errs := k.KeepLock(ctx, "key")
	faults.block.Store(true)
	var started time.Time
	select {
	case started = <-faults.entered:
	case <-ctx.Done():
		t.Fatal("renewal never reached the blocked API")
	}
	select {
	case err := <-errs:
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Less(t, time.Since(started), k.Cfg.RenewDeadline+500*time.Millisecond)
	case <-done:
		t.Fatal("done must not bypass the subscriber's lease-loss error handling")
	case <-ctx.Done():
		t.Fatal("renewal did not respect its deadline")
	}
	select {
	case <-done:
		t.Fatal("lease-loss notification must use only the error channel")
	default:
	}
	old, err := client.CoordinationV1().Leases("test").Get(ctx, leaseName("key"), metav1.GetOptions{})
	require.NoError(t, err)
	faults.block.Store(false)
	ok, err = k.Lock(ctx, "key", []byte("pod"))
	require.NoError(t, err)
	require.True(t, ok)
	current, err := client.CoordinationV1().Leases("test").Get(ctx, leaseName("key"), metav1.GetOptions{})
	require.NoError(t, err)
	require.NotEqual(t, *old.Spec.HolderIdentity, *current.Spec.HolderIdentity)
}

func TestRenewalUsesUpdateFastPath(t *testing.T) {
	client := fake.NewClientset()
	k := testLocker(t, client)
	ok, err := k.Lock(t.Context(), "key", []byte("pod"))
	require.NoError(t, err)
	require.True(t, ok)
	client.ClearActions()
	require.Eventually(t, func() bool {
		updates := 0
		for _, action := range client.Actions() {
			if action.GetVerb() == "update" {
				updates++
			}
		}
		return updates >= 3
	}, time.Second, 10*time.Millisecond)
	for _, action := range client.Actions() {
		require.NotEqual(t, "get", action.GetVerb(), "renewal should use the resourceVersion from the previous write")
	}
}

func TestCancelPendingAcquisitionPreservesForeignLease(t *testing.T) {
	foreign := testLease("key", "foreign-pod", time.Now())
	client := fake.NewClientset(foreign)
	k := testLocker(t, client)
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { _, err := k.Lock(ctx, "key", []byte("pod")); result <- err }()
	require.Eventually(t, func() bool {
		for _, action := range client.Actions() {
			if action.GetVerb() == "get" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	_, err := k.Lock(t.Context(), "key", []byte("duplicate"))
	require.ErrorContains(t, err, "already active")
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("acquisition did not stop on cancellation")
	}
	require.NoError(t, k.Unlock(t.Context(), "key"))
	require.NoError(t, k.Stop())
	current, err := client.CoordinationV1().Leases("test").Get(t.Context(), foreign.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, foreign.Spec, current.Spec)
	for _, action := range client.Actions() {
		require.NotEqual(t, "delete", action.GetVerb())
	}
	_, err = k.Lock(t.Context(), "new", nil)
	require.Error(t, err)
}

func TestUnlockProtectsReplacementAndReportsAPIErrors(t *testing.T) {
	for _, mode := range []string{"foreign", "conflict", "failure", "owned"} {
		t.Run(mode, func(t *testing.T) {
			client := fake.NewClientset()
			k := testLocker(t, client)
			ok, err := k.Lock(t.Context(), "key", []byte("pod"))
			require.NoError(t, err)
			require.True(t, ok)
			session := k.locks["key"]
			session.cancel(context.Canceled)
			<-session.stopped
			api := client.CoordinationV1().Leases("test")
			lease, err := api.Get(t.Context(), leaseName("key"), metav1.GetOptions{})
			require.NoError(t, err)
			lease.UID, lease.ResourceVersion = "uid-original", "rv-original"
			if mode == "foreign" {
				identity := "new-process"
				lease.Spec.HolderIdentity = &identity
			}
			_, err = api.Update(t.Context(), lease, metav1.UpdateOptions{})
			require.NoError(t, err)
			calls := 0
			wantErr := errors.New("API unavailable")
			client.PrependReactor("delete", "leases", func(action clienttesting.Action) (bool, runtime.Object, error) {
				calls++
				options := action.(clienttesting.DeleteAction).GetDeleteOptions()
				require.NotNil(t, options.Preconditions)
				require.Equal(t, lease.UID, *options.Preconditions.UID)
				require.Equal(t, lease.ResourceVersion, *options.Preconditions.ResourceVersion)
				if mode == "conflict" {
					return true, nil, apierrors.NewConflict(coordinationv1.Resource("leases"), lease.Name, wantErr)
				}
				if mode == "failure" {
					return true, nil, wantErr
				}
				return false, nil, nil
			})
			err = k.Unlock(t.Context(), "key")
			if mode == "failure" {
				require.ErrorIs(t, err, wantErr)
			} else {
				require.NoError(t, err)
			}
			if mode == "foreign" {
				require.Zero(t, calls)
			} else {
				require.Equal(t, 1, calls)
			}
			_, err = api.Get(t.Context(), lease.Name, metav1.GetOptions{})
			require.Equal(t, mode == "owned", apierrors.IsNotFound(err))
		})
	}
}

type interceptedClient struct {
	*fake.Clientset
	leases coordinationclient.LeaseInterface
}

func (c interceptedClient) CoordinationV1() coordinationclient.CoordinationV1Interface {
	return interceptedCoordination{CoordinationV1Interface: c.Clientset.CoordinationV1(), leases: c.leases}
}

type interceptedCoordination struct {
	coordinationclient.CoordinationV1Interface
	leases coordinationclient.LeaseInterface
}

func (c interceptedCoordination) Leases(string) coordinationclient.LeaseInterface { return c.leases }

type blockingLeases struct {
	coordinationclient.LeaseInterface
	block   atomic.Bool
	entered chan time.Time
}

func (l *blockingLeases) Update(ctx context.Context, lease *coordinationv1.Lease, opts metav1.UpdateOptions) (*coordinationv1.Lease, error) {
	if l.block.Load() {
		select {
		case l.entered <- time.Now():
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return l.LeaseInterface.Update(ctx, lease, opts)
}
