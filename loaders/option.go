package loaders

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Option func(TargetLoader)

func WithRegistry(reg *prometheus.Registry) Option {
	return func(l TargetLoader) {
		l.RegisterMetrics(reg)
	}
}
