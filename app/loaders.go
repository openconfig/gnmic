package app

import "github.com/karimra/gnmic/loaders"

func (a *App) startLoader() {
	ldCfg, _ := a.Config.GetLoader()
	if ldType, ok := ldCfg["type"]; ok {
		if in, ok := loaders.Loaders[ldType.(string)]; ok {
			ld := in()
			err := ld.Init(a.ctx, ldCfg)
			if err != nil {
				a.Logger.Printf("failed to init loader: %v", err)
				return
			}
			for targetOp := range ld.Start(a.ctx) {
				for _, add := range targetOp.Add {
					err = a.collector.AddTarget(add)
					if err != nil {
						a.Logger.Printf("failed addting target %q: %v", add.Name, err)
						continue
					}
					go a.collector.TargetSubscribeStream(a.ctx, add.Name)
				}
				for _, del := range targetOp.Del {
					a.collector.DeleteTarget(a.ctx, del)
				}
			}
			a.Logger.Printf("target loader stopped")
			return
		}
	}
	a.Logger.Printf("missing type field under loader config")
}
