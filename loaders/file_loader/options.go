package file_loader

import (
	"github.com/karimra/gnmic/types"
	"github.com/prometheus/client_golang/prometheus"
)

func (f *fileLoader) RegisterMetrics(reg *prometheus.Registry) {
	if !f.cfg.EnableMetrics {
		return
	}
	if err := registerMetrics(reg); err != nil {
		f.logger.Printf("failed to register metrics: %v", err)
	}
}

func (f *fileLoader) WithActions(acts map[string]map[string]interface{}) {
	f.actionsConfig = acts
}

func (f *fileLoader) WithTargetsDefaults(fn func(tc *types.TargetConfig) error) {
	f.targetConfigFn = fn
}
