package app

import (
	"context"
	"fmt"

	"github.com/fullstorydev/grpcurl"
	"github.com/karimra/gnmic/target"
	"github.com/karimra/gnmic/types"
)

// initTarget initializes a new target given its name.
// it assumes that the config struct is already locked.
func (a *App) initTarget(name string) error {
	// config is already reader-locked
	if tc, ok := a.Config.Targets[name]; ok {
		if !a.targetExists(name) {
			t := target.NewTarget(tc)
			for _, subName := range tc.Subscriptions {
				if sub, ok := a.Config.Subscriptions[subName]; ok {
					t.Subscriptions[subName] = sub
				}
			}
			if len(t.Subscriptions) == 0 {
				for _, sub := range a.Config.Subscriptions {
					t.Subscriptions[sub.Name] = sub
				}
			}
			err := a.parseProtoFiles(t)
			if err != nil {
				return err
			}
			a.operLock.Lock()
			a.Targets[t.Config.Name] = t
			a.operLock.Unlock()
		}
		return nil
	}
	return fmt.Errorf("unknown target")
}

func (a *App) stopTarget(ctx context.Context, name string) error {
	if a.Targets == nil {
		return nil
	}
	if !a.targetExists(name) {
		return fmt.Errorf("target %q does not exist", name)
	}
	a.operLock.Lock()
	defer a.operLock.Unlock()
	a.Logger.Printf("stopping target %q", name)
	t := a.Targets[name]
	t.Stop()
	delete(a.Targets, name)
	if a.locker == nil {
		return nil
	}
	return a.locker.Unlock(ctx, a.targetLockKey(name))
}

func (a *App) DeleteTarget(ctx context.Context, name string) error {
	if a.Targets == nil {
		return nil
	}
	if !a.targetConfigExists(name) {
		return fmt.Errorf("target %q does not exist", name)
	}
	a.configLock.Lock()
	delete(a.Config.Targets, name)
	a.configLock.Unlock()
	a.Logger.Printf("target %q deleted from config", name)
	// delete from oper map
	a.operLock.Lock()
	defer a.operLock.Unlock()
	if cfn, ok := a.targetsLockFn[name]; ok {
		cfn()
	}
	if t, ok := a.Targets[name]; ok {
		t.Stop()
		delete(a.Targets, name)
		if a.locker != nil {
			return a.locker.Unlock(ctx, a.targetLockKey(name))
		}
	}
	return nil
}

// AddTargetConfig adds a *TargetConfig to the configuration map
func (a *App) AddTargetConfig(tc *types.TargetConfig) {
	a.Logger.Printf("adding target %+v", tc)
	if tc.BufferSize <= 0 {
		tc.BufferSize = a.Config.TargetBufferSize
	}
	if tc.RetryTimer <= 0 {
		tc.RetryTimer = a.Config.Retry
	}

	if _, ok := a.Config.Targets[tc.Name]; ok {
		return
	}

	a.configLock.Lock()
	defer a.configLock.Unlock()
	a.Config.Targets[tc.Name] = tc
}

func (a *App) parseProtoFiles(t *target.Target) error {
	if len(t.Config.ProtoFiles) == 0 {
		t.RootDesc = a.rootDesc
		return nil
	}
	a.Logger.Printf("target %q loading proto files...", t.Config.Name)
	descSource, err := grpcurl.DescriptorSourceFromProtoFiles(t.Config.ProtoDirs, t.Config.ProtoFiles...)
	if err != nil {
		a.Logger.Printf("failed to load proto files: %v", err)
		return err
	}
	t.RootDesc, err = descSource.FindSymbol("Nokia.SROS.root")
	if err != nil {
		a.Logger.Printf("target %q could not get symbol 'Nokia.SROS.root': %v", t.Config.Name, err)
		return err
	}
	a.Logger.Printf("target %q loaded proto files", t.Config.Name)
	return nil
}

func (a *App) targetExists(name string) bool {
	a.operLock.RLock()
	_, ok := a.Targets[name]
	a.operLock.RUnlock()
	return ok
}

func (a *App) targetConfigExists(name string) bool {
	a.configLock.RLock()
	_, ok := a.Config.Targets[name]
	a.configLock.RUnlock()
	return ok
}
