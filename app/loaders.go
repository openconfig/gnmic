package app

import (
	"context"
	"time"

	"github.com/karimra/gnmic/loaders"
)

func (a *App) startLoader(ctx context.Context) {
START:
	ldCfg, _ := a.Config.GetLoader()
	if ldType, ok := ldCfg["type"]; ok {
		ldTypeS, ok := ldType.(string)
		if !ok {
			a.Logger.Printf("field 'type' not a string, found a %T", ldType)
			return
		}
		ticker := time.NewTicker(10 * time.Second)
		if !a.inCluster() {
			ticker.Stop()
			goto INIT
		}
		// wait for instance to become the leader
		for range ticker.C {
			if a.isLeader {
				ticker.Stop()
				break
			}
		}
	INIT:
		a.Logger.Printf("initializing loader type %q", ldTypeS)
		if in, ok := loaders.Loaders[ldTypeS]; ok {
			ld := in()
			err := ld.Init(ctx, ldCfg)
			if err != nil {
				a.Logger.Printf("failed to init loader: %v", err)
				return
			}
			a.Logger.Printf("starting loader type %q", ldTypeS)
			for targetOp := range ld.Start(ctx) {
				for _, add := range targetOp.Add {
					a.Config.SetTargetConfigDefaults(add)
					err = a.collector.AddTarget(add)
					if err != nil {
						a.Logger.Printf("failed adding target %q: %v", add.Name, err)
						continue
					}
					go a.collector.TargetSubscribeStream(ctx, add.Name)
				}
				for _, del := range targetOp.Del {
					err = a.collector.DeleteTarget(ctx, del)
					if err != nil {
						a.Logger.Printf("failed deleting target %q: %v", del, err)
						continue
					}
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
		a.Logger.Printf("unknown loader type %q", ldTypeS)
		return
	}
	a.Logger.Printf("missing type field under loader config")
}
