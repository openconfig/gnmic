// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestKeepLockCancellationCompletesWithoutErrorReceiver(t *testing.T) {
	k := testLocker(t, fake.NewClientset())
	ctx, cancel := context.WithCancel(t.Context())
	ok, err := k.Lock(ctx, "key", []byte("pod"))
	require.NoError(t, err)
	require.True(t, ok)
	done, errs := k.KeepLock(ctx, "key")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("KeepLock retained a goroutine waiting for the canceled caller")
	}
	select {
	case err := <-errs:
		t.Fatalf("caller cancellation reported as renewal failure: %v", err)
	default:
	}
	require.NoError(t, k.Unlock(t.Context(), "key"))
}

func TestLockAfterKeepLockCancellationDoesNotWaitForExpiry(t *testing.T) {
	client := fake.NewClientset()
	k := testLocker(t, client)
	ctx, cancel := context.WithCancel(t.Context())
	ok, err := k.Lock(ctx, "key", []byte("pod"))
	require.NoError(t, err)
	require.True(t, ok)
	done, _ := k.KeepLock(ctx, "key")
	cancel()
	<-done
	ctx, cancel = context.WithTimeout(t.Context(), k.Cfg.LeaseDuration/2)
	defer cancel()
	ok, err = k.Lock(ctx, "key", []byte("pod"))
	require.NoError(t, err, "a canceled session of this instance must not delay reacquisition until its Lease expires")
	require.True(t, ok)
	k.mu.Lock()
	identity := k.locks["key"].identity
	k.mu.Unlock()
	lease, err := client.CoordinationV1().Leases("test").Get(t.Context(), leaseName("key"), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, identity, *lease.Spec.HolderIdentity)
}

func TestLeaseCacheInitializationRespectsCancellation(t *testing.T) {
	k := &k8sLocker{Cfg: &config{Namespace: "test"}, clientset: fake.NewClientset()}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, k.startLeaseCache(ctx), context.Canceled)
	select {
	case <-k.cacheDone:
	default:
		t.Fatal("informer outlived failed initialization")
	}
}

func TestKeepLockMissingSessionReportsFailure(t *testing.T) {
	k := testLocker(t, fake.NewClientset())
	done, errs := k.KeepLock(t.Context(), "missing")
	select {
	case err := <-errs:
		require.ErrorContains(t, err, "not active")
	case <-done:
		t.Fatal("missing ownership must stop collection through the error path")
	case <-time.After(time.Second):
		t.Fatal("missing ownership was not reported")
	}
}
