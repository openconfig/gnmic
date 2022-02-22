package docker_loader

import (
	"github.com/karimra/gnmic/types"
	"github.com/prometheus/client_golang/prometheus"
)

func (d *dockerLoader) RegisterMetrics(reg *prometheus.Registry) {
	if !d.cfg.EnableMetrics {
		return
	}
	if err := registerMetrics(reg); err != nil {
		d.logger.Printf("failed to register metrics: %v", err)
	}
}

func (d *dockerLoader) WithActions(acts map[string]map[string]interface{}) {
	d.actionsConfig = acts
}

func (d *dockerLoader) WithTargetsDefaults(fn func(tc *types.TargetConfig) error) {
	d.targetConfigFn = fn
}
