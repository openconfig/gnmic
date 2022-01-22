package outputs

import (
	"log"

	"github.com/karimra/gnmic/types"
	"github.com/prometheus/client_golang/prometheus"
)

type Option func(Output)

func WithLogger(logger *log.Logger) Option {
	return func(o Output) {
		o.SetLogger(logger)
	}
}

func WithEventProcessors(eps map[string]map[string]interface{},
	log *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) Option {
	return func(o Output) {
		o.SetEventProcessors(eps, log, tcs, acts)
	}
}

func WithRegister(reg *prometheus.Registry) Option {
	return func(o Output) {
		o.RegisterMetrics(reg)
	}
}

func WithName(name string) Option {
	return func(o Output) {
		o.SetName(name)
	}
}

func WithClusterName(name string) Option {
	return func(o Output) {
		o.SetClusterName(name)
	}
}

func WithTargetsConfig(tcs map[string]*types.TargetConfig) Option {
	return func(o Output) {
		o.SetTargetsConfig(tcs)
	}
}
