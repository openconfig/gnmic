// © 2023 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import "github.com/prometheus/client_golang/prometheus"

const (
	namespace = "gnmic"
	subsystem = "prometheus_write_output"
)

var prometheusWriteNumberOfSentMsgs = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "number_of_prometheus_write_msgs_sent_success_total",
	Help:      "Number of msgs successfully sent by gnmic prometheus_write output",
})

var prometheusWriteNumberOfFailSendMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "number_of_prometheus_write_msgs_sent_fail_total",
	Help:      "Number of failed msgs sent by gnmic prometheus_write output",
}, []string{"reason"})

var prometheusWriteSendDuration = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "msg_send_duration_ns",
	Help:      "gnmic prometheus_write output send duration in ns",
})

var prometheusWriteNumberOfSentMetadataMsgs = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "number_of_prometheus_write_metadata_msgs_sent_success_total",
	Help:      "Number of metadata msgs successfully sent by gnmic prometheus_write output",
})

var prometheusWriteNumberOfFailSendMetadataMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "number_of_prometheus_write_metadata_msgs_sent_fail_total",
	Help:      "Number of failed metadata msgs sent by gnmic prometheus_write output",
}, []string{"reason"})

var prometheusWriteMetadataSendDuration = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "metadata_msg_send_duration_ns",
	Help:      "gnmic prometheus_write output metadata send duration in ns",
})

func initMetrics() {
	// data msgs metrics
	prometheusWriteNumberOfSentMsgs.Add(0)
	prometheusWriteNumberOfFailSendMsgs.WithLabelValues("").Add(0)
	prometheusWriteSendDuration.Set(0)
	// metadata msgs metrics
	prometheusWriteNumberOfSentMetadataMsgs.Add(0)
	prometheusWriteNumberOfFailSendMetadataMsgs.WithLabelValues("").Add(0)
	prometheusWriteMetadataSendDuration.Set(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(prometheusWriteNumberOfSentMsgs); err != nil {
		return err
	}
	if err = reg.Register(prometheusWriteNumberOfFailSendMsgs); err != nil {
		return err
	}
	if err = reg.Register(prometheusWriteSendDuration); err != nil {
		return err
	}
	if err = reg.Register(prometheusWriteNumberOfSentMetadataMsgs); err != nil {
		return err
	}
	if err = reg.Register(prometheusWriteNumberOfFailSendMetadataMsgs); err != nil {
		return err
	}
	if err = reg.Register(prometheusWriteMetadataSendDuration); err != nil {
		return err
	}
	return nil
}
