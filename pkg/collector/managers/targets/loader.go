package targets_manager

import (
	"context"
	"fmt"

	"github.com/openconfig/gnmic/pkg/loaders"
)

func (tm *TargetsManager) initLoader(cfg map[string]any) (loaders.TargetLoader, error) {
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

func (tm *TargetsManager) startLoader(ctx context.Context, loader loaders.TargetLoader) {
	ch := loader.Start(ctx)
	for {
		select {
		case <-ctx.Done():
			tm.logger.Info("loader stopped")
			return
		case targetOp := <-ch:
			for _, add := range targetOp.Add {
				_, err := tm.store.Set("targets", add.Name, add)
				if err != nil {
					tm.logger.Error("failed to add target from loader", "error", err, "target", add.Name)
				}
			}
			for _, del := range targetOp.Del {
				_, _, err := tm.store.Delete("targets", del)
				if err != nil {
					tm.logger.Error("failed to delete target from loader", "error", err, "target", del)
				}
			}
		}
	}
}
