// © 2025 NVIDIA Corporation
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of NVIDIA's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package otlp_output

import (
	"github.com/prometheus/client_golang/prometheus"
)

var otlpNumberOfSentEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "otlp_output",
	Name:      "number_of_sent_events_total",
	Help:      "Number of events successfully sent to OTLP collector",
}, []string{"output_name"})

var otlpNumberOfFailedEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "otlp_output",
	Name:      "number_of_failed_events_total",
	Help:      "Number of events that failed to send to OTLP collector",
}, []string{"output_name", "reason"})

var otlpSendDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "gnmic",
	Subsystem: "otlp_output",
	Name:      "send_duration_seconds",
	Help:      "Duration of sending batches to OTLP collector",
}, []string{"output_name"})

var otlpRejectedDataPoints = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "otlp_output",
	Name:      "rejected_data_points_total",
	Help:      "Number of data points rejected by OTLP collector (PartialSuccess)",
}, []string{"output_name"})
