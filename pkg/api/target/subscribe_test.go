// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"fmt"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"
)

// TestSubscriptionsConcurrentAccess is a regression test for
// https://github.com/openconfig/gnmic/issues/835: concurrent writes to
// Target.Subscriptions (from the collector's targets manager) raced with
// locked reads in SubscribeClientStates. Run with -race.
func TestSubscriptionsConcurrentAccess(t *testing.T) {
	tg := NewTarget(&types.TargetConfig{Name: "t1"})

	var wg sync.WaitGroup
	const iterations = 1000

	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tg.SetSubscriptionConfig(&types.SubscriptionConfig{
				Name: fmt.Sprintf("sub%d", i%10),
			})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tg.DeleteSubscriptionConfig(fmt.Sprintf("sub%d", i%10))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = tg.SubscribeClientStates()
		}
	}()
	wg.Wait()
}
