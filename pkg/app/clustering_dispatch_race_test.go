package app

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
)

type alwaysLockedLocker struct {
	lockers.Locker
}

func (alwaysLockedLocker) IsLocked(context.Context, string) (bool, error) {
	// yield so the writer goroutine runs mid-iteration
	time.Sleep(time.Microsecond)
	return true, nil
}

// The loader writes Config.Targets under configLock while the leader's
// dispatch loop ranges over it; the loop must not read the map unguarded.
func TestDispatchTargetsOnceSafeAgainstConcurrentLoaderWrites(t *testing.T) {
	a := New()
	t.Cleanup(a.Cfn)
	a.Config.Clustering = &config.Clustering{ClusterName: "test", TargetsWatchTimer: time.Second}
	a.locker = alwaysLockedLocker{}
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("device-%d", i)
		a.Config.Targets[name] = &types.TargetConfig{Name: name}
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			name := fmt.Sprintf("added-%d", i)
			a.configLock.Lock()
			a.Config.Targets[name] = &types.TargetConfig{Name: name}
			a.configLock.Unlock()
		}
	}()
	for i := 0; i < 5; i++ {
		a.dispatchTargetsOnce(t.Context())
	}
	close(stop)
	wg.Wait()
}
