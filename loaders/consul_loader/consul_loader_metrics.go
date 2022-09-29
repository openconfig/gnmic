// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package consul_loader

import "github.com/prometheus/client_golang/prometheus"

var consulLoaderLoadedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "consul_loader",
	Name:      "number_of_loaded_targets",
	Help:      "Number of new targets successfully loaded",
}, []string{"loader_type"})

var consulLoaderDeletedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "consul_loader",
	Name:      "number_of_deleted_targets",
	Help:      "Number of targets successfully deleted",
}, []string{"loader_type"})

var consulLoaderWatchError = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "consul_loader",
	Name:      "number_of_watch_errors",
	Help:      "Number of watch errors",
}, []string{"loader_type", "error"})

func initMetrics() {
	consulLoaderLoadedTargets.WithLabelValues(loaderType).Set(0)
	consulLoaderDeletedTargets.WithLabelValues(loaderType).Set(0)
	consulLoaderWatchError.WithLabelValues(loaderType, "").Add(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(consulLoaderLoadedTargets); err != nil {
		return err
	}
	if err = reg.Register(consulLoaderDeletedTargets); err != nil {
		return err
	}
	return reg.Register(consulLoaderWatchError)
}
