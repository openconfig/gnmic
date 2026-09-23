// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package targets_manager

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestDeleteTargetMetrics verifies that deleting a target removes every
// per-target metric series for that target (regression test for phantom series
// and unbounded metric cardinality across add/delete churn), while leaving
// other targets' series untouched.
func TestDeleteTargetMetrics(t *testing.T) {
	tm := &TargetsManager{stats: newTargetsStats()}

	// Populate series for two targets so we can prove only the deleted one is
	// removed.
	for _, name := range []string{"r1", "r2"} {
		tm.stats.targetUPMetric.WithLabelValues(name).Set(1)
		tm.stats.targetConnStateMetric.WithLabelValues(name).Set(3)
		tm.stats.subscribeResponseReceived.WithLabelValues(name, "sub1").Add(5)
		tm.stats.droppedSubscribeResponses.WithLabelValues(name, "sub1").Add(1)
		tm.stats.subscriptionFailedCount.WithLabelValues(name, "sub1", subscriptionRequestErrorTypeGRPC).Inc()
	}

	// Sanity: two series in every vector before deletion.
	assertCount(t, "before targetUPMetric", tm.stats.targetUPMetric, 2)
	assertCount(t, "before targetConnStateMetric", tm.stats.targetConnStateMetric, 2)
	assertCount(t, "before subscribeResponseReceived", tm.stats.subscribeResponseReceived, 2)
	assertCount(t, "before droppedSubscribeResponses", tm.stats.droppedSubscribeResponses, 2)
	assertCount(t, "before subscriptionFailedCount", tm.stats.subscriptionFailedCount, 2)

	tm.deleteTargetMetrics("r1")

	// Exactly one series (r2) must remain in every vector.
	assertCount(t, "after targetUPMetric", tm.stats.targetUPMetric, 1)
	assertCount(t, "after targetConnStateMetric", tm.stats.targetConnStateMetric, 1)
	assertCount(t, "after subscribeResponseReceived", tm.stats.subscribeResponseReceived, 1)
	assertCount(t, "after droppedSubscribeResponses", tm.stats.droppedSubscribeResponses, 1)
	assertCount(t, "after subscriptionFailedCount", tm.stats.subscriptionFailedCount, 1)

	// The surviving series must be r2, not r1.
	if got := testutil.ToFloat64(tm.stats.targetUPMetric.WithLabelValues("r2")); got != 1 {
		t.Fatalf("r2 gnmic_target_up: got %v, want 1", got)
	}
}

func assertCount(t *testing.T, what string, c prometheus.Collector, want int) {
	t.Helper()
	if got := testutil.CollectAndCount(c); got != want {
		t.Fatalf("%s: series count = %d, want %d", what, got, want)
	}
}
