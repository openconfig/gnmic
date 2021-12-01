package consul_loader

import (
	"github.com/karimra/gnmic/types"
	"github.com/prometheus/client_golang/prometheus"
)

func (c *consulLoader) RegisterMetrics(reg *prometheus.Registry) {
	if !c.cfg.EnableMetrics && reg != nil {
		return
	}
	if err := registerMetrics(reg); err != nil {
		c.logger.Printf("failed to register metrics: %v", err)
	}
}

func (c *consulLoader) WithActions(acts map[string]map[string]interface{}) {
	// l.actionsConfig = acts
}

func (c *consulLoader) WithTargetsDefaults(func(tc *types.TargetConfig) error) {}
