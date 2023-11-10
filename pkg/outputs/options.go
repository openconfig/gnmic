// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package outputs

import (
	"log"

	"github.com/openconfig/gnmic/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
)

type Option func(Output) error

func WithLogger(logger *log.Logger) Option {
	return func(o Output) error {
		o.SetLogger(logger)
		return nil
	}
}

func WithEventProcessors(eps map[string]map[string]interface{},
	log *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) Option {
	return func(o Output) error {
		return  o.SetEventProcessors(eps, log, tcs, acts)
	}
}

func WithRegistry(reg *prometheus.Registry) Option {
	return func(o Output) error {
		o.RegisterMetrics(reg)
		return nil
	}
}

func WithName(name string) Option {
	return func(o Output) error {
		o.SetName(name)
		return nil
	}
}

func WithClusterName(name string) Option {
	return func(o Output) error {
		o.SetClusterName(name)
		return nil
	}
}

func WithTargetsConfig(tcs map[string]*types.TargetConfig) Option {
	return func(o Output) error {
		o.SetTargetsConfig(tcs)
		return nil
	}
}
