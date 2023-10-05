// © 2023 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_output

import "github.com/prometheus/client_golang/prometheus"

const (
	namespace = "gnmic"
	subsystem = "prometheus_output"
)

var prometheusNumberOfMetrics = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "number_of_prometheus_metrics_total",
		Help:      "Number of metrics stored by the prometheus output",
	})

var prometheusNumberOfCachedMetrics = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "number_of_prometheus_cached_metrics_total",
		Help:      "Number of metrics cached by the prometheus output",
	})

func (p *prometheusOutput) initMetrics() {
	if p.cfg.CacheConfig == nil {
		prometheusNumberOfMetrics.Set(0)
		return
	}
	prometheusNumberOfCachedMetrics.Set(0)
}

func (p *prometheusOutput) registerMetrics(reg *prometheus.Registry) error {
	p.initMetrics()
	if p.cfg.CacheConfig == nil {
		return reg.Register(prometheusNumberOfMetrics)
	}
	return reg.Register(prometheusNumberOfCachedMetrics)
}
