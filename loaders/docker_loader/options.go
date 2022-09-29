// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package docker_loader

import (
	"github.com/openconfig/gnmic/types"
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
