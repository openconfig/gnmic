package http_loader

import (
	"github.com/karimra/gnmic/types"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *httpLoader) RegisterMetrics(reg *prometheus.Registry) {
	if !h.cfg.EnableMetrics {
		return
	}
	if err := registerMetrics(reg); err != nil {
		h.logger.Printf("failed to register metrics: %v", err)
	}
}

func (h *httpLoader) WithActions(acts map[string]map[string]interface{}) {
	h.actionsConfig = acts
}

func (h *httpLoader) WithTargetsDefaults(fn func(tc *types.TargetConfig) error) {
	h.targetConfigFn = fn
}
