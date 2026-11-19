package targets_manager

import (
	"context"
	"fmt"

	"github.com/openconfig/gnmic/pkg/loaders"
)

func (tm *TargetsManager) initLoader(cfg map[string]any) (loaders.Loader, error) {
	loaderType, ok := cfg["type"].(string)
	if !ok {
		return nil, fmt.Errorf("loader type is required")
	}
	for _, lt := range loaders.LoadersTypes {
		if lt == loaderType {
			init, ok := loaders.Loaders[loaderType]
			if !ok {
				return nil, fmt.Errorf("unknown loader type %q", loaderType)
			}
			loader := init()
			return loader, nil
		}
	}
	return nil, fmt.Errorf("unknown loader type %q", loaderType)
}

func (tm *TargetsManager) startLoader(ctx context.Context, loader loaders.Loader) {
	ch := loader.Start(ctx)
	for {
		select {
		case <-ctx.Done():
			tm.logger.Info("loader stopped")
			return
		case op := <-ch:
			for _, add := range op.Add {
				_, err := tm.store.Config.Set("targets", add.Name, add)
				if err != nil {
					tm.logger.Error("failed to add target from loader", "error", err, "target", add.Name)
				}
			}
			for _, del := range op.Del {
				_, _, err := tm.store.Config.Delete("targets", del)
				if err != nil {
					tm.logger.Error("failed to delete target from loader", "error", err, "target", del)
				}
			}
			for _, add := range op.SubAdd {
				_, err := tm.store.Config.Set("subscriptions", add.Name, add)
				if err != nil {
					tm.logger.Error("failed to add subscription from loader", "error", err, "subscription", add.Name)
				}
			}
			for _, del := range op.SubDel {
				_, _, err := tm.store.Config.Delete("subscriptions", del)
				if err != nil {
					tm.logger.Error("failed to delete subscription from loader", "error", err, "subscription", del)
				}
			}
		}
	}
}
