package app

import (
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
)

// TestAddTargetConfig_ConcurrentRace detects the concurrent map read/write
// that caused fatal crashes in production (exit code 2).
// Run with -race to verify: go test -race ./pkg/app/... -run TestAddTargetConfig_ConcurrentRace
func TestAddTargetConfig_ConcurrentRace(t *testing.T) {
	a := &App{
		Config:     config.New(),
		configLock: new(sync.RWMutex),
		Logger:     slog.New(slog.DiscardHandler),
	}

	// pre-populate so DeleteTarget has something to delete
	for i := 0; i < 10; i++ {
		a.Config.Targets[fmt.Sprintf("target-%d", i)] = &types.TargetConfig{
			Name: fmt.Sprintf("target-%d", i),
		}
	}

	var wg sync.WaitGroup
	const goroutines = 50

	// goroutines concurrently adding targets
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a.AddTargetConfig(&types.TargetConfig{
				Name: fmt.Sprintf("new-target-%d", i),
			})
		}(i)
	}

	// goroutines concurrently deleting targets — races with the unprotected read in AddTargetConfig
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a.DeleteTarget(t.Context(), fmt.Sprintf("target-%d", i))
		}(i)
	}

	// goroutines concurrently reading targets — also races with writes
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a.AddTargetConfig(&types.TargetConfig{
				Name: fmt.Sprintf("new-target-%d", i), // duplicate name: tests the early-return read path
			})
		}(i)
	}

	wg.Wait()

	// Verify all new targets were added exactly once
	for i := 0; i < goroutines; i++ {
		name := fmt.Sprintf("new-target-%d", i)
		a.configLock.RLock()
		_, ok := a.Config.Targets[name]
		a.configLock.RUnlock()
		if !ok {
			t.Errorf("target %q missing after concurrent add", name)
		}
	}
}

// TestMetricsLoopVsLoaderWrite reproduces:
//
//	fatal error: concurrent map iteration and map write
//
// The metrics loop (registerTargetMetrics) iterates a.Config.Targets under configLock.RLock.
// The loader non-clustered path wrote a.Config.Targets[add.Name] = add without any lock,
// bypassing the RLock/Lock protocol entirely.
// Run with: go test -race ./pkg/app/... -run TestMetricsLoopVsLoaderWrite
func TestMetricsLoopVsLoaderWrite(t *testing.T) {
	a := &App{
		Config:     config.New(),
		configLock: new(sync.RWMutex),
		operLock:   new(sync.RWMutex),
		Targets:    make(map[string]*target.Target),
		Logger:     slog.New(slog.DiscardHandler),
	}

	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("target-%d", i)
		a.Config.Targets[name] = &types.TargetConfig{Name: name}
	}

	var wg sync.WaitGroup
	const iterations = 100

	// mirrors registerTargetMetrics ticker body exactly
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.configLock.RLock()
			for _, tc := range a.Config.Targets {
				a.operLock.RLock()
				_, _ = a.Targets[tc.Name]
				a.operLock.RUnlock()
			}
			a.configLock.RUnlock()
		}()
	}

	// mirrors startLoader non-clustered path bug: write without configLock
	// races with the map iteration in the metrics goroutines above
	// Run with -race to see: fatal error: concurrent map iteration and map write
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// tc := &types.TargetConfig{Name: fmt.Sprintf("new-target-%d", i)}
			// a.Config.Targets[tc.Name] = tc // no lock: reproduces the race
			a.AddTargetConfig(&types.TargetConfig{Name: fmt.Sprintf("new-target-%d", i)})
		}(i)
	}

	wg.Wait()
}
