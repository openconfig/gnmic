// © 2025 Drew Elliott
//
// This code is a Contribution to the gNMIc project ("Work") made under the Apache License 2.0.
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
