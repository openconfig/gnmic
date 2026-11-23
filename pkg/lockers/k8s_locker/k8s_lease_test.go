// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"bytes"
	"context"
	"log/slog"
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
	"github.com/openconfig/gnmic/pkg/logging"
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
			Name: leaseName(key), Namespace: "test", Labels: leaseLabels(key, []byte(value)),
			Annotations: map[string]string{origKeyName: key, origValueName: value},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: ptr.To("foreign-session"), LeaseDurationSeconds: ptr.To(int32(15)),
			RenewTime: &metav1.MicroTime{Time: renewed},
		},
	}
}

func TestLeaseNamesAndOriginalIdentity(t *testing.T) {
	legacyKey := "gnmic/default-cluster/targets/router-1"
	require.Equal(t, "gnmic-default-cluster-targets-router-1", leaseName(legacyKey))

	keys := []string{"gnmic/CLUSTER/targets/SIM-LEAF-00-00", "192.0.2.1:57400", "[2001:db8::1]:57400", "设备/端口", strings.Repeat("long/", 100), "A/B", "A-B"}
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

func TestLegacyLeaseMetadataCompatibility(t *testing.T) {
	key, value := "gnmic/default-cluster/targets/router-1", "gnmic-0"
	lease := testLease(key, value, time.Now())
	delete(lease.Annotations, origValueName)
	client := fake.NewClientset(lease)
	k := testLocker(t, client)

	values, err := k.List(t.Context(), "gnmic/default-cluster/targets")
	require.NoError(t, err)
	require.Equal(t, map[string]string{key: value}, values)
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	ok, err := k.Lock(ctx, key, []byte("gnmic-1"))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, ok)
	leases, err := client.CoordinationV1().Leases("test").List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, leases.Items, 1)
	require.Equal(t, legacyLeaseName(key), leases.Items[0].Name)

	ok, err = k.Lock(t.Context(), "gnmic/default-cluster/leader", []byte(value))
	require.NoError(t, err)
	require.True(t, ok)
	created, err := client.CoordinationV1().Leases("test").Get(t.Context(), leaseName("gnmic/default-cluster/leader"), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, value, created.Labels[legacyLeaseName("gnmic/default-cluster/leader")])
	require.Equal(t, value, created.Annotations[origValueName])
}

func TestLeaseCacheFiltersAndTracksOwnership(t *testing.T) {
	now := time.Now()
	active := testLease("cluster/target", "pod-a", now)
	nested := testLease("cluster/target-2/sub", "pod-e", now)
	expired := testLease("cluster/expired", "pod-b", now.Add(-time.Minute))
	missing := testLease("cluster/missing", "pod-c", now)
	missing.Spec.LeaseDurationSeconds = nil
	other := testLease("other/target", "pod-d", now)
	client := fake.NewClientset(active, nested, expired, missing, other)
	k := testLocker(t, client)
	dirs, err := leaseKeyDirs(nested)
	require.NoError(t, err)
	require.Equal(t, []string{"cluster", "cluster/target-2"}, dirs)
	indexed, err := k.leaseIndex.ByIndex(leaseKeyDirIndex, "cluster")
	require.NoError(t, err)
	require.Len(t, indexed, 4)
	cluster := map[string]string{"cluster/target": "pod-a", "cluster/target-2/sub": "pod-e"}
	before := len(client.Actions())
	for range 100 {
		for prefix, want := range map[string]map[string]string{
			"cluster/":         cluster,
			"cluster/target":   cluster,
			"cluster/tar":      cluster,
			"clu":              cluster,
			"cluster/target-2": {"cluster/target-2/sub": "pod-e"},
			"":                 {"cluster/target": "pod-a", "cluster/target-2/sub": "pod-e", "other/target": "pod-d"},
		} {
			values, err := k.List(t.Context(), prefix)
			require.NoError(t, err)
			require.Equal(t, want, values, prefix)
		}
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
	_, err = api.Update(t.Context(), active, metav1.UpdateOptions{})
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
	require.Equal(t, config{Namespace: "default", LeaseDuration: 15 * time.Second, RenewDeadline: 10 * time.Second, RetryPeriod: 2 * time.Second}, *k.Cfg)
	require.NotContains(t, k.String(), "qps")
	qps, burst := k.Cfg.clientLimits()
	require.Equal(t, float32(100), qps)
	require.Equal(t, 200, burst)
	qps, burst = (&config{QPS: 20, Burst: -1}).clientLimits()
	require.Equal(t, float32(20), qps)
	require.Equal(t, 200, burst)

	for name, cfg := range map[string]config{
		"sub-second lease":             {LeaseDuration: 500 * time.Millisecond},
		"renew deadline reaches lease": {RenewDeadline: 15 * time.Second},
		"renew deadline within jitter": {RenewDeadline: time.Second, RetryPeriod: time.Second},
		"negative retry period":        {RetryPeriod: -time.Second},
		"conflicting renew alias":      {RenewDeadline: 4 * time.Second, RenewPeriod: 5 * time.Second},
		"conflicting retry alias":      {RetryPeriod: time.Second, RetryTimer: 2 * time.Second},
		"alias fallback conflicts":     {RenewPeriod: 14 * time.Second, RetryPeriod: 10 * time.Second},
	} {
		require.Error(t, (&k8sLocker{Cfg: &cfg, logger: logging.DiscardLogger()}).setDefaults(), name)
	}
}

func TestKubernetesLockerCorrectsLegacyTimings(t *testing.T) {
	const second = time.Second
	for name, tc := range map[string]struct {
		cfg                 config
		lease, renew, retry time.Duration
		warning             bool
	}{
		"short lease":         {cfg: config{LeaseDuration: 5 * second}, lease: 5 * second, renew: 5 * second * 2 / 3, retry: 1250 * time.Millisecond},
		"fractional lease":    {cfg: config{LeaseDuration: 2500 * time.Millisecond}, lease: 2 * second, renew: 2 * second * 2 / 3, retry: 500 * time.Millisecond, warning: true},
		"valid aliases":       {cfg: config{LeaseDuration: 10 * second, RenewPeriod: 5 * second, RetryTimer: 2 * second}, lease: 10 * second, renew: 5 * second, retry: 2 * second},
		"alias reaches lease": {cfg: config{LeaseDuration: 10 * second, RenewPeriod: 8 * second}, lease: 10 * second, renew: 10 * second * 2 / 3, retry: 2 * second, warning: true},
		"alias above lease":   {cfg: config{LeaseDuration: 10 * second, RenewPeriod: 12 * second}, lease: 10 * second, renew: 10 * second * 2 / 3, retry: 2 * second, warning: true},
		"alias within jitter": {cfg: config{LeaseDuration: 10 * second, RenewPeriod: 2 * second, RetryTimer: 2 * second}, lease: 10 * second, renew: 10 * second * 2 / 3, retry: 2 * second, warning: true},
	} {
		t.Run(name, func(t *testing.T) {
			var logs bytes.Buffer
			k := &k8sLocker{Cfg: &tc.cfg, logger: slog.New(slog.NewTextHandler(&logs, nil))}
			for range 2 {
				require.NoError(t, k.setDefaults())
				require.Equal(t, tc.lease, k.Cfg.LeaseDuration)
				require.Equal(t, tc.renew, k.Cfg.RenewDeadline)
				require.Equal(t, tc.retry, k.Cfg.RetryPeriod)
			}
			require.Equal(t, tc.warning, strings.Contains(logs.String(), "level=WARN"), logs.String())
		})
	}
}
