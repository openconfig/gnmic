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
		case targetOp, ok := <-ch:
			if !ok {
				tm.logger.Info("loader channel closed")
				return
			}
			tm.applyLoaderOp(targetOp)
		}
	}
}

// applyLoaderOp writes a loader operation to the config store.
// loaders.Diff reports a changed target in both Add and Del. Its Del entry is
// skipped: setting the existing key is an update, which apply handles in
// place, while deleting it after the set would drop the target.
func (tm *TargetsManager) applyLoaderOp(op *loaders.TargetOperation) {
	if op == nil {
		return
	}
	for _, del := range op.Del {
		if _, ok := op.Add[del]; ok {
			continue
		}
		_, _, err := tm.store.Config.Delete("targets", del)
		if err != nil {
			tm.logger.Error("failed to delete target from loader", "error", err, "target", del)
		}
	}
	for _, add := range op.Add {
		// The loader keeps add as its last-seen config. Store a copy so an API
		// change to the stored target does not alter what the loader diffs.
		_, err := tm.store.Config.Set("targets", add.Name, add.DeepCopy())
		if err != nil {
			tm.logger.Error("failed to add target from loader", "error", err, "target", add.Name)
		}
	}
}
