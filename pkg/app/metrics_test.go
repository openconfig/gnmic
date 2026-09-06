// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestUpdateClusterMetricsSkipsGaugesOnLockerListError(t *testing.T) {
	a := New()
	defer a.Cfn()
	a.Config.ClusterName = "c"
	a.Config.Clustering = &config.Clustering{InstanceName: "instance-1"}
	lk := &clusterMetricsLocker{
		leader: map[string]string{"gnmic/c/leader": "instance-1"},
		targets: map[string]string{
			"gnmic/c/targets/device-a": "instance-1",
			"gnmic/c/targets/device-b": "instance-1",
			"gnmic/c/targets/device-c": "instance-2",
		},
	}
	a.locker = lk

	a.updateClusterMetrics(context.Background())
	if got := testutil.ToFloat64(clusterIsLeader); got != 1 {
		t.Fatalf("is_leader after success = %v, want 1", got)
	}
	if got := testutil.ToFloat64(clusterNumberOfLockedTargets); got != 2 {
		t.Fatalf("locked targets after success = %v, want 2", got)
	}

	leaderFails := testutil.ToFloat64(clusterLockerListFailed.WithLabelValues("leader"))
	targetFails := testutil.ToFloat64(clusterLockerListFailed.WithLabelValues("targets"))
	lk.leaderErr = errors.New("leader list failed")
	lk.targetErr = errors.New("target list failed")
	a.updateClusterMetrics(context.Background())
	if got := testutil.ToFloat64(clusterIsLeader); got != 1 {
		t.Fatalf("is_leader after both failures = %v, want previous 1", got)
	}
	if got := testutil.ToFloat64(clusterNumberOfLockedTargets); got != 2 {
		t.Fatalf("locked targets after both failures = %v, want previous 2", got)
	}
	if got := testutil.ToFloat64(clusterLockerListFailed.WithLabelValues("leader")); got != leaderFails+1 {
		t.Fatalf("leader failures = %v, want %v", got, leaderFails+1)
	}
	if got := testutil.ToFloat64(clusterLockerListFailed.WithLabelValues("targets")); got != targetFails+1 {
		t.Fatalf("target failures = %v, want %v", got, targetFails+1)
	}

	lk.leaderErr = errors.New("leader list failed")
	lk.targetErr = nil
	lk.targets = map[string]string{}
	a.updateClusterMetrics(context.Background())
	if got := testutil.ToFloat64(clusterIsLeader); got != 1 {
		t.Fatalf("is_leader after leader-only failure = %v, want previous 1", got)
	}
	if got := testutil.ToFloat64(clusterNumberOfLockedTargets); got != 0 {
		t.Fatalf("locked targets after empty success = %v, want 0", got)
	}
}

type clusterMetricsLocker struct {
	lockers.Locker
	leader    map[string]string
	leaderErr error
	targets   map[string]string
	targetErr error
}

func (l *clusterMetricsLocker) List(_ context.Context, prefix string) (map[string]string, error) {
	if strings.Contains(prefix, "/targets") {
		return l.targets, l.targetErr
	}
	return l.leader, l.leaderErr
}
