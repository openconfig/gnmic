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

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/prometheus/client_golang/prometheus"
)

type OutputOptions struct {
	Name            string
	ClusterName     string
	Logger          *log.Logger
	EventProcessors map[string]map[string]any
	TargetsConfig   map[string]*types.TargetConfig
	Actions         map[string]map[string]any
	Registry        *prometheus.Registry
}

type Option func(*OutputOptions) error

func WithLogger(logger *log.Logger) Option {
	return func(o *OutputOptions) error {
		o.Logger = logger
		return nil
	}
}

func WithEventProcessors(eps map[string]map[string]any, acts map[string]map[string]any) Option {
	return func(o *OutputOptions) error {
		o.EventProcessors = eps
		o.Actions = acts
		return nil
	}
}

func WithRegistry(reg *prometheus.Registry) Option {
	return func(o *OutputOptions) error {
		o.Registry = reg
		return nil
	}
}

func WithName(name string) Option {
	return func(o *OutputOptions) error {
		o.Name = name
		return nil
	}
}

func WithClusterName(name string) Option {
	return func(o *OutputOptions) error {
		o.ClusterName = name
		return nil
	}
}

func WithTargetsConfig(tcs map[string]*types.TargetConfig) Option {
	return func(o *OutputOptions) error {
		o.TargetsConfig = tcs
		return nil
	}
}
