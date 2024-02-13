// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package loaders

import (
	"context"
	"log"

	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/openconfig/gnmic/pkg/api/types"
)

// TargetLoader discovers a set of target configurations for gNMIc to run RPCs against.
// RunOnce should return a map of target configs and is meant to be used with Unary RPCs.
// Start runs a goroutine in the background that updates added/removed target configs on the
// returned channel.
type TargetLoader interface {
	// Init initializes the target loader given the config, logger and options
	Init(ctx context.Context, cfg map[string]interface{}, l *log.Logger, opts ...Option) error
	// RunOnce runs the loader only once, returning a map of target configs
	RunOnce(ctx context.Context) (map[string]*types.TargetConfig, error)
	// Start starts the target loader, running periodic polls or a long watch.
	// It returns a channel of TargetOperation from which the function caller can
	// receive the added/removed target configs
	Start(context.Context) chan *TargetOperation
	// RegsiterMetrics registers the loader metrics with the provided registry
	RegisterMetrics(*prometheus.Registry)
	// WithActions passes the actions configuration to the target loader
	WithActions(map[string]map[string]interface{})
	// WithTargetsDefaults passes a callback function that sets the target config defaults
	WithTargetsDefaults(func(tc *types.TargetConfig) error)
}

type Initializer func() TargetLoader

var Loaders = map[string]Initializer{}

var LoadersTypes = []string{
	"file",
	"consul",
	"docker",
	"http",
}

func Register(name string, initFn Initializer) {
	Loaders[name] = initFn
}

type TargetOperation struct {
	Add map[string]*types.TargetConfig
	Del []string
}

func DecodeConfig(src, dst interface{}) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     dst,
		},
	)
	if err != nil {
		return err
	}
	return decoder.Decode(src)
}

func Diff(m1, m2 map[string]*types.TargetConfig) *TargetOperation {
	result := &TargetOperation{
		Add: make(map[string]*types.TargetConfig, 0),
		Del: make([]string, 0),
	}
	if len(m1) == 0 {
		for n, t := range m2 {
			result.Add[n] = t
		}
		return result
	}
	if len(m2) == 0 {
		for name := range m1 {
			result.Del = append(result.Del, name)
		}
		return result
	}
	for n, t := range m2 {
		if _, ok := m1[n]; !ok {
			result.Add[n] = t
		}
	}
	for n := range m1 {
		if _, ok := m2[n]; !ok {
			result.Del = append(result.Del, n)
		}
	}
	return result
}
