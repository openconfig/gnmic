package app

import (
	"context"
	"time"

	"github.com/karimra/gnmic/loaders"
)

func (a *App) startLoader(ctx context.Context) {
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
START:
	ldCfg, err := a.Config.GetLoader()
	if err != nil {
		a.Logger.Printf("failed getting loader config")
	}
	if ldCfg == nil {
		return
	}
	ldTypeS := ldCfg["type"].(string)
	a.Logger.Printf("initializing loader type %q", ldTypeS)

	ld := loaders.Loaders[ldTypeS]()
	err = ld.Init(ctx, ldCfg)
	if err != nil {
		a.Logger.Printf("failed to init loader type %q: %v", ldTypeS, err)
		return
	}
	a.Logger.Printf("starting loader type %q", ldTypeS)
	for targetOp := range ld.Start(ctx) {
		for _, del := range targetOp.Del {
			// not clustered, delete local target
			if !a.inCluster() {
				err = a.collector.DeleteTarget(ctx, del)
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
			a.Config.SetTargetConfigDefaults(add)
			// not clustered, add target and subscribe
			if !a.inCluster() {
				a.Config.Targets[add.Name] = add
				err = a.collector.AddTarget(add)
				if err != nil {
					a.Logger.Printf("failed adding target %q: %v", add.Name, err)
					continue
				}
				a.wg.Add(1)
				go a.collector.TargetSubscribeStream(ctx, add.Name)
				continue
			}
			// clustered, dispatch
			a.m.Lock()
			a.Config.Targets[add.Name] = add
			err = a.dispatchTarget(a.ctx, add)
			if err != nil {
				a.Logger.Printf("failed dispatching target %q: %v", add.Name, err)
			}
			a.m.Unlock()
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
