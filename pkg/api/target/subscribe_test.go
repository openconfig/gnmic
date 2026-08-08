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
			name := fmt.Sprintf("sub%d", i%10)
			tg.SetSubscription(name, &types.SubscriptionConfig{Name: name})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tg.SetSubscription(fmt.Sprintf("sub%d", i%10), nil)
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
