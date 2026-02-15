package targets_manager

import (
	"fmt"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
)

func (tm *TargetsManager) isClustering() (*config.Clustering, bool, error) {
	clusterCfg, ok, err := tm.store.Config.Get("clustering", "clustering")
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	clustering, ok := clusterCfg.(*config.Clustering)
	if ok {
		return clustering, true, nil
	}
	return nil, false, nil
}

func (tm *TargetsManager) amIAssigned(name string) bool {
	if !tm.incluster {
		return true // run all targets in standalone mode
	}
	tm.mas.RLock()
	defer tm.mas.RUnlock()
	_, ok := tm.assignments[name]
	return ok
}

func (tm *TargetsManager) setAssigned(name string, v bool) {
	tm.mas.Lock()
	if tm.assignments == nil {
		tm.assignments = map[string]struct{}{}
	}
	if v {
		tm.assignments[name] = struct{}{}
	} else {
		delete(tm.assignments, name)
	}
	tm.mas.Unlock()
	//
	if v {
		cfg, ok, err := tm.store.Config.Get("targets", name)
		if err != nil {
			tm.logger.Error("failed to get target", "target", name, "error", err)
			return
		}
		if ok {
			tcfg, tok := cfg.(*types.TargetConfig)
			if tok {
				tm.apply(name, tcfg)
			} else {
				tm.logger.Error("target config is not a types.TargetConfig", "target", name, "config", cfg)
			}
		} else {
			tm.logger.Error(" assignedtarget config not found", "target", name)
		}
	} else {
		tm.remove(name)
	}

}

func (tm *TargetsManager) targetLockKey(target string) string {
	return fmt.Sprintf("gnmic/%s/targets/%s", tm.clustering.ClusterName, target)
}
