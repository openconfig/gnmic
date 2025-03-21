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
	"time"

	"github.com/openconfig/gnmic/pkg/loaders"
)

func (a *App) startLoader(ctx context.Context) {
	if len(a.Config.Loader) == 0 {
		return
	}
	if a.inCluster() {
		ticker := time.NewTicker(time.Second)
		// wait for instance to become the leader
		for range ticker.C {
			if a.isLeader {
				ticker.Stop()
				break
			}
		}
	}
	ldTypeS := a.Config.Loader["type"].(string)
START:
	a.Logger.Printf("initializing loader type %q", ldTypeS)

	ld := loaders.Loaders[ldTypeS]()
	err := ld.Init(ctx, a.Config.Loader, a.Logger,
		loaders.WithRegistry(a.reg),
		loaders.WithActions(a.Config.Actions),
		loaders.WithTargetsDefaults(a.Config.SetTargetConfigDefaults),
	)
	if err != nil {
		a.Logger.Printf("failed to init loader type %q: %v", ldTypeS, err)
		return
	}
	a.Logger.Printf("starting loader type %q", ldTypeS)
	for targetOp := range ld.Start(ctx) {
		// do deletes first, since target change equates to delete+add
		for _, del := range targetOp.Del {
			// not clustered, delete local target
			if !a.inCluster() {
				err = a.DeleteTarget(ctx, del)
				if err != nil {
					a.Logger.Printf("failed deleting target %q: %v", del, err)
				}
				continue
			}
			// clustered, delete target in all instances of the cluster
			err = a.deleteTarget(ctx, del)
			if err != nil {
				a.Logger.Printf("failed to delete target %q: %v", del, err)
			}
		}

		var limiter *time.Ticker
		if a.Config.LocalFlags.SubscribeBackoff > 0 {
			limiter = time.NewTicker(a.Config.LocalFlags.SubscribeBackoff)
		}

		for _, add := range targetOp.Add {
			err = a.Config.SetTargetConfigDefaults(add)
			if err != nil {
				a.Logger.Printf("failed parsing new target configuration %#v: %v", add, err)
				continue
			}
			// not clustered, add target and subscribe
			if !a.inCluster() {
				a.Config.Targets[add.Name] = add
				a.AddTargetConfig(add)
				a.wg.Add(1)
				go a.TargetSubscribeStream(ctx, add)
				if limiter != nil {
					<-limiter.C
				}
				continue
			}
			// clustered, dispatch
			a.configLock.Lock()
			a.Config.Targets[add.Name] = add
			err = a.dispatchTarget(ctx, add)
			if err != nil {
				a.Logger.Printf("failed dispatching target %q: %v", add.Name, err)
			}
			a.configLock.Unlock()
		}

		if limiter != nil {
			limiter.Stop()
		}
	}
	a.Logger.Printf("target loader stopped")
	select {
	case <-ctx.Done():
		return
	default:
		goto START
	}
}

func (a *App) startLoaderProxy(ctx context.Context) {
	if len(a.Config.Loader) == 0 {
		return
	}
	ldTypeS := a.Config.Loader["type"].(string)
START:
	a.Logger.Printf("initializing loader type %q", ldTypeS)

	ld := loaders.Loaders[ldTypeS]()
	err := ld.Init(ctx, a.Config.Loader, a.Logger,
		loaders.WithRegistry(a.reg),
		loaders.WithActions(a.Config.Actions),
		loaders.WithTargetsDefaults(a.Config.SetTargetConfigDefaults),
	)
	if err != nil {
		a.Logger.Printf("failed to init loader type %q: %v", ldTypeS, err)
		return
	}
	a.Logger.Printf("starting loader type %q", ldTypeS)
	for targetOp := range ld.Start(ctx) {
		// do deletes first since target change is delete+add
		for _, del := range targetOp.Del {
			// clustered, delete target in all instances of the cluster
			a.configLock.Lock()
			delete(a.Config.Targets, del)
			a.configLock.Unlock()
			a.operLock.Lock()
			t, ok := a.Targets[del]
			if ok {
				err = t.Close()
				if err != nil {
					a.Logger.Printf("failed to stop target %s: %v", del, err)
				}
				delete(a.Targets, del)
			}
			a.operLock.Unlock()
		}
		for _, add := range targetOp.Add {
			err = a.Config.SetTargetConfigDefaults(add)
			if err != nil {
				a.Logger.Printf("failed parsing new target configuration %#v: %v", add, err)
				continue
			}

			a.configLock.Lock()
			a.Config.Targets[add.Name] = add
			a.configLock.Unlock()
		}
	}
	a.Logger.Printf("target loader stopped")
	select {
	case <-ctx.Done():
		return
	default:
		goto START
	}
}
