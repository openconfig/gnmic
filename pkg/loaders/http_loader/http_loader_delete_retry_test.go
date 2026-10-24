// © 2026 Supranett.
//
// SPDX-License-Identifier: Apache-2.0

package http_loader

import (
	"testing"

	"github.com/openconfig/gnmic/pkg/actions"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/loaders"
)

// A deletion that fails must stay in the loader's diff so the next poll retries
// it. updateTargets removed the name from lastTargets regardless of whether the
// delete action had succeeded, so a single failure left the target stranded on
// the member with an empty diff on every subsequent poll.
func TestUpdateTargetsRetainsFailedDeleteForRetry(t *testing.T) {
	for _, tc := range []struct {
		name        string
		deleteFails bool
	}{
		{name: "delete succeeds", deleteFails: false},
		{name: "delete fails", deleteFails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hl := newTestLoader(t)
			hl.delActions = []actions.Action{
				&fakeAction{name: "del", fail: tc.deleteFails},
			}
			hl.numActions = len(hl.delActions)

			// The target is known to the loader, then disappears from discovery.
			hl.lastTargets["t1"] = &types.TargetConfig{Name: "t1", Address: "10.0.0.1"}

			opChan := make(chan *loaders.TargetOperation, 1)
			hl.updateTargets(t.Context(), map[string]*types.TargetConfig{}, opChan)

			_, stillTracked := hl.lastTargets["t1"]
			if tc.deleteFails && !stillTracked {
				t.Fatal("lastTargets no longer contains t1 after a failed delete: " +
					"the next poll computes an empty diff and the deletion is never retried")
			}
			if !tc.deleteFails && stillTracked {
				t.Fatal("lastTargets still contains t1 after a successful delete")
			}
		})
	}
}
