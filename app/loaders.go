package app

import (
	"context"
	"time"

	"github.com/karimra/gnmic/loaders"
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
			err = a.deleteTarget(del)
			if err != nil {
				a.Logger.Printf("failed to delete target %q: %v", del, err)
			}
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
				continue
			}
			// clustered, dispatch
			a.configLock.Lock()
			a.Config.Targets[add.Name] = add
			err = a.dispatchTarget(a.ctx, add)
			if err != nil {
				a.Logger.Printf("failed dispatching target %q: %v", add.Name, err)
			}
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
