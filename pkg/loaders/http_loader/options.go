// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package http_loader

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/openconfig/gnmic/pkg/types"
)

func (h *httpLoader) RegisterMetrics(reg *prometheus.Registry) {
	if !h.cfg.EnableMetrics {
		return
	}
	if reg == nil {
		h.logger.Printf("ERR: metrics enabled but main registry is not initialized, enable main metrics under `api-server`")
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
