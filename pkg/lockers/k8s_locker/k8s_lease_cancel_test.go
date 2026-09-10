// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
