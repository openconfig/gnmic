package http_loader

import (
	"github.com/karimra/gnmic/types"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *httpLoader) RegisterMetrics(reg *prometheus.Registry) {
	if !h.cfg.EnableMetrics && reg != nil {
		return
	}
	if err := registerMetrics(reg); err != nil {
		h.logger.Printf("failed to register metrics: %v", err)
	}
}

func (h *httpLoader) WithActions(acts map[string]map[string]interface{}) {
	//	l.actionsConfig = acts
}

func (h *httpLoader) WithTargetsDefaults(func(tc *types.TargetConfig) error) {}
