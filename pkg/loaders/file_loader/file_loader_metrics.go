// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package file_loader

import "github.com/prometheus/client_golang/prometheus"

var fileLoaderLoadedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "file_loader",
	Name:      "number_of_loaded_targets",
	Help:      "Number of new targets successfully loaded",
}, []string{"loader_type"})

var fileLoaderDeletedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "file_loader",
	Name:      "number_of_deleted_targets",
	Help:      "Number of targets successfully deleted",
}, []string{"loader_type"})

var fileLoaderFailedFileRead = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_loader",
	Name:      "number_of_failed_file_reads",
	Help:      "Number of times gnmic failed to read the file",
}, []string{"loader_type", "error"})

var fileLoaderFileReadTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_loader",
	Name:      "number_of_file_read_attempts_total",
	Help:      "Number of times the loader attempted to read the file",
}, []string{"loader_type"})

var fileLoaderFileReadDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "file_loader",
	Name:      "file_read_duration_ns",
	Help:      "Duration of file read in ns",
}, []string{"loader_type"})

func initMetrics() {
	fileLoaderLoadedTargets.WithLabelValues(loaderType).Set(0)
	fileLoaderDeletedTargets.WithLabelValues(loaderType).Set(0)
	fileLoaderFailedFileRead.WithLabelValues(loaderType, "").Add(0)
	fileLoaderFileReadTotal.WithLabelValues(loaderType).Add(0)
	fileLoaderFileReadDuration.WithLabelValues(loaderType).Set(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(fileLoaderLoadedTargets); err != nil {
		return err
	}
	if err = reg.Register(fileLoaderDeletedTargets); err != nil {
		return err
	}
	if err = reg.Register(fileLoaderFailedFileRead); err != nil {
		return err
	}
	if err = reg.Register(fileLoaderFileReadTotal); err != nil {
		return err
	}
	if err = reg.Register(fileLoaderFileReadDuration); err != nil {
		return err
	}
	return nil
}
