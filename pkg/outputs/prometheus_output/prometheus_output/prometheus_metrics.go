// © 2023 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_output

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "gnmic"
	subsystem = "prometheus_output"
)

var registerMetricsOnce sync.Once

var prometheusNumberOfMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "number_of_prometheus_metrics_total",
		Help:      "Number of metrics stored by the prometheus output",
	}, []string{"name"})

var prometheusNumberOfCachedMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "number_of_prometheus_cached_metrics_total",
		Help:      "Number of metrics cached by the prometheus output",
	}, []string{"name"})

func (p *prometheusOutput) initMetrics() {
	if p.cfg.CacheConfig == nil {
		prometheusNumberOfMetrics.WithLabelValues(p.cfg.Name).Set(0)
		return
	}
	prometheusNumberOfCachedMetrics.WithLabelValues(p.cfg.Name).Set(0)
}

func (p *prometheusOutput) registerMetrics() error {
	if !p.cfg.EnableMetrics {
		return nil
	}
	if p.reg == nil {
		return nil
	}
	var err error
	registerMetricsOnce.Do(func() {
		if p.cfg.CacheConfig == nil {
			err = p.reg.Register(prometheusNumberOfMetrics)
			return
		}
		err = p.reg.Register(prometheusNumberOfCachedMetrics)
	})
	p.initMetrics()
	return err
}
