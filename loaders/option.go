// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package loaders

import (
	"github.com/karimra/gnmic/types"
	"github.com/prometheus/client_golang/prometheus"
)

type Option func(TargetLoader)

func WithRegistry(reg *prometheus.Registry) Option {
	return func(l TargetLoader) {
		if reg == nil {
			return
		}
		l.RegisterMetrics(reg)
	}
}

func WithActions(acts map[string]map[string]interface{}) Option {
	return func(l TargetLoader) {
		if len(acts) == 0 {
			return
		}
		l.WithActions(acts)
	}
}

func WithTargetsDefaults(fn func(tc *types.TargetConfig) error) Option {
	return func(l TargetLoader) {
		l.WithTargetsDefaults(fn)
	}
}
