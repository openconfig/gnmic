// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	"github.com/openconfig/gnmic/pkg/lockers"
)

func testLocker(t *testing.T, client kubernetes.Interface) *k8sLocker {
	t.Helper()
	k := lockers.Lockers["k8s"]().(*k8sLocker)
	k.Cfg = &config{Namespace: "test", LeaseDuration: 2 * time.Second, RenewDeadline: 350 * time.Millisecond, RetryPeriod: 50 * time.Millisecond}
	require.NoError(t, k.setDefaults())
	k.clientset = client
	require.NoError(t, k.startLeaseCache(t.Context()))
	t.Cleanup(func() { require.NoError(t, k.Stop()) })
	return k
}

func testLease(key, value string, renewed time.Time) *coordinationv1.Lease {
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: leaseName(key), Namespace: "test", Labels: map[string]string{"app": "gnmic"},
			Annotations: map[string]string{origKeyName: key, origValueName: value},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: ptr.To("foreign-session"), LeaseDurationSeconds: ptr.To(int32(15)),
			RenewTime: &metav1.MicroTime{Time: renewed},
		},
	}
}

func TestLeaseNamesAndOriginalIdentity(t *testing.T) {
	keys := []string{"gnmic/CLUSTER/targets/SIM-LEAF-00-00", "192.0.2.1:57400", "[2001:db8::1]:57400", "设备/端口", strings.Repeat("long/", 100), "a/b", "a-b"}
	names := make(map[string]bool)
	for _, key := range keys {
		name := leaseName(key)
		require.Empty(t, validation.IsDNS1123Subdomain(name), key)
		require.False(t, names[name], "distinct keys collided")
		names[name] = true
		require.Equal(t, name, leaseName(key))
	}
	client := fake.NewClientset()
	k := testLocker(t, client)
	key, value := keys[0], "Collector-"+strings.Repeat("A", 100)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	ok, err := k.Lock(ctx, key, []byte(value))
	require.NoError(t, err)
	require.True(t, ok)
	lease, err := client.CoordinationV1().Leases("test").Get(t.Context(), leaseName(key), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"app": "gnmic"}, lease.Labels)
	require.Equal(t, key, lease.Annotations[origKeyName])
	require.Equal(t, value, lease.Annotations[origValueName])
	require.NotEqual(t, value, *lease.Spec.HolderIdentity)
	require.Eventually(t, func() bool {
		values, err := k.List(t.Context(), "gnmic/CLUSTER/")
		return err == nil && values[key] == value
	}, time.Second, 10*time.Millisecond)
}

func TestLeaseCacheFiltersAndTracksOwnership(t *testing.T) {
	now := time.Now()
	active := testLease("cluster/target", "pod-a", now)
	expired := testLease("cluster/expired", "pod-b", now.Add(-time.Minute))
	missing := testLease("cluster/missing", "pod-c", now)
	missing.Spec.LeaseDurationSeconds = nil
	other := testLease("other/target", "pod-d", now)
	client := fake.NewClientset(active, expired, missing, other)
	k := testLocker(t, client)
	before := len(client.Actions())
	for range 100 {
		values, err := k.List(t.Context(), "cluster/")
		require.NoError(t, err)
		require.Equal(t, map[string]string{"cluster/target": "pod-a"}, values)
		for key, want := range map[string]bool{"cluster/target": true, "cluster/expired": false, "cluster/missing": false, "unknown": false} {
			locked, err := k.IsLocked(t.Context(), key)
			require.NoError(t, err)
			require.Equal(t, want, locked, key)
		}
	}
	for _, action := range client.Actions()[before:] {
		require.NotContains(t, []string{"get", "list"}, action.GetVerb(), "ownership queries must read the cache")
	}
	api := client.CoordinationV1().Leases("test")
	active.Annotations[origValueName] = "pod-new"
	_, err := api.Update(t.Context(), active, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		values, _ := k.List(t.Context(), "cluster/")
		return values["cluster/target"] == "pod-new"
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, api.Delete(t.Context(), active.Name, metav1.DeleteOptions{}))
	require.Eventually(t, func() bool {
		locked, _ := k.IsLocked(t.Context(), "cluster/target")
		return !locked
	}, time.Second, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = k.List(ctx, "")
	require.ErrorIs(t, err, context.Canceled)
}

func TestLeaseValidity(t *testing.T) {
	now := time.Now()
	for _, mutate := range []func(*coordinationv1.Lease){
		func(l *coordinationv1.Lease) { l.Spec.HolderIdentity = nil },
		func(l *coordinationv1.Lease) { l.Spec.HolderIdentity = ptr.To("") },
		func(l *coordinationv1.Lease) { l.Spec.RenewTime = nil },
		func(l *coordinationv1.Lease) { l.Spec.LeaseDurationSeconds = nil },
		func(l *coordinationv1.Lease) { l.Spec.LeaseDurationSeconds = ptr.To(int32(0)) },
		func(l *coordinationv1.Lease) { l.Spec.LeaseDurationSeconds = ptr.To(int32(-1)) },
		func(l *coordinationv1.Lease) { l.Spec.RenewTime = &metav1.MicroTime{Time: now.Add(-15 * time.Second)} },
	} {
		lease := testLease("key", "owner", now)
		require.True(t, validLease(lease, now))
		mutate(lease)
		require.False(t, validLease(lease, now))
	}
}

func TestKubernetesLockerConfig(t *testing.T) {
	k := lockers.Lockers["k8s"]().(*k8sLocker)
	require.NoError(t, k.setDefaults())
	require.Equal(t, 15*time.Second, k.Cfg.LeaseDuration)
	require.Equal(t, 10*time.Second, k.Cfg.RenewDeadline)
	require.Equal(t, 2*time.Second, k.Cfg.RetryPeriod)
	require.Equal(t, float32(100), k.Cfg.QPS)
	require.Equal(t, 200, k.Cfg.Burst)
	for _, mutate := range []func(*config){
		func(c *config) { c.LeaseDuration = 1500 * time.Millisecond },
		func(c *config) { c.LeaseDuration = -time.Second },
		func(c *config) { c.RenewDeadline = 15 * time.Second },
		func(c *config) { c.RenewDeadline = time.Second },
		func(c *config) { c.RetryPeriod = -time.Second },
		func(c *config) { c.QPS = -1 },
		func(c *config) { c.Burst = -1 },
	} {
		cfg := *k.Cfg
		mutate(&cfg)
		require.Error(t, (&k8sLocker{Cfg: &cfg}).setDefaults())
	}
	for _, field := range []string{"renew-period", "retry-timer"} {
		require.ErrorContains(t, k.Init(t.Context(), map[string]any{field: "1s"}), field)
	}
}
