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
