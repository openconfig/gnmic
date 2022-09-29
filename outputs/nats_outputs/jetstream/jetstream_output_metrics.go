// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package jetstream_output

import "github.com/prometheus/client_golang/prometheus"

var jetStreamNumberOfSentMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "jetstream_output",
	Name:      "number_of_jetstream_msgs_sent_success_total",
	Help:      "Number of msgs successfully sent by gnmic jetstream output",
}, []string{"publisher_id", "subject"})

var jetStreamNumberOfSentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "jetstream_output",
	Name:      "number_of_written_jetstream_bytes_total",
	Help:      "Number of bytes written by gnmic jetstream output",
}, []string{"publisher_id", "subject"})

var jetStreamNumberOfFailSendMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "jetstream_output",
	Name:      "number_of_jetstream_msgs_sent_fail_total",
	Help:      "Number of failed msgs sent by gnmic jetstream output",
}, []string{"publisher_id", "reason"})

var jetStreamSendDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "jetstream_output",
	Name:      "msg_send_duration_ns",
	Help:      "gnmic jetstream output send duration in ns",
}, []string{"publisher_id"})

func initMetrics() {
	jetStreamNumberOfSentMsgs.WithLabelValues("", "").Add(0)
	jetStreamNumberOfSentBytes.WithLabelValues("", "").Add(0)
	jetStreamNumberOfFailSendMsgs.WithLabelValues("", "").Add(0)
	jetStreamSendDuration.WithLabelValues("").Set(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(jetStreamNumberOfSentMsgs); err != nil {
		return err
	}
	if err = reg.Register(jetStreamNumberOfSentBytes); err != nil {
		return err
	}
	if err = reg.Register(jetStreamNumberOfFailSendMsgs); err != nil {
		return err
	}
	if err = reg.Register(jetStreamSendDuration); err != nil {
		return err
	}
	return nil
}
