// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/fullstorydev/grpcurl"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
)

// initTarget initializes a new target given its name.
// it assumes that the configLock as well as the operLock
// are acquired.
func (a *App) initTarget(tc *types.TargetConfig) (*target.Target, error) {
	t, ok := a.Targets[tc.Name]
	if !ok {
		t := target.NewTarget(tc)
		for _, subName := range tc.Subscriptions {
			if sub, ok := a.Config.Subscriptions[subName]; ok {
				t.Subscriptions[subName] = sub
			}
		}
		if len(t.Subscriptions) == 0 {
			for n, sub := range a.Config.Subscriptions {
				t.Subscriptions[n] = sub
			}
		}
		err := a.parseProtoFiles(t)
		if err != nil {
			return nil, err
		}
		a.Targets[t.Config.Name] = t
		return t, nil
	}
	return t, nil
}

func (a *App) stopTarget(ctx context.Context, name string) error {
	if a.Targets == nil {
		return nil
	}
	a.operLock.Lock()
	defer a.operLock.Unlock()
	if _, ok := a.Targets[name]; !ok {
		return fmt.Errorf("target %q does not exist", name)
	}

	a.Logger.Printf("stopping target %q", name)
	t := a.Targets[name]
	t.StopSubscriptions()
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
	if a.c != nil {
		a.c.DeleteTarget(name)
	}
	if t, ok := a.Targets[name]; ok {
		delete(a.Targets, name)
		t.Close()
		if a.locker != nil {
			return a.locker.Unlock(ctx, a.targetLockKey(name))
		}
	}
	return nil
}

// UpdateTargetConfig updates the subscriptions for an existing target
func (a *App) UpdateTargetSubscription(ctx context.Context, name string, subs []string) error {
	a.configLock.Lock()
	for _, subName := range subs {
		if _, ok := a.Config.Subscriptions[subName]; !ok {
			a.configLock.Unlock()
			return fmt.Errorf("subscription %q does not exist", subName)
		}
	}
	targetConfig := a.Config.Targets[name]
	targetConfig.Subscriptions = subs
	a.configLock.Unlock()

	if err := a.stopTarget(ctx, name); err != nil {
		return err
	}

	go a.TargetSubscribeStream(ctx, targetConfig)
	return nil
}

// AddTargetConfig adds a *TargetConfig to the configuration map
func (a *App) AddTargetConfig(tc *types.TargetConfig) {
	a.Logger.Printf("adding target %s", tc)
	_, ok := a.Config.Targets[tc.Name]
	if ok {
		return
	}
	if tc.BufferSize <= 0 {
		tc.BufferSize = a.Config.TargetBufferSize
	}
	if tc.RetryTimer <= 0 {
		tc.RetryTimer = a.Config.Retry
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

func (a *App) targetConfigExists(name string) bool {
	a.configLock.RLock()
	_, ok := a.Config.Targets[name]
	a.configLock.RUnlock()
	return ok
}
