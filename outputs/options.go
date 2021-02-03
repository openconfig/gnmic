package outputs

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

type Option func(Output)

func WithLogger(logger *log.Logger) Option {
	return func(o Output) {
		o.SetLogger(logger)
	}
}

func WithEventProcessors(eps map[string]map[string]interface{}, log *log.Logger) Option {
	return func(o Output) {
		o.SetEventProcessors(eps, log)
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
