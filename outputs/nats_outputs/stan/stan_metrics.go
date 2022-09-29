// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package stan_output

import "github.com/prometheus/client_golang/prometheus"

var StanNumberOfSentMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "stan_output",
	Name:      "number_of_stan_msgs_sent_success_total",
	Help:      "Number of msgs successfully sent by gnmic stan output",
}, []string{"publisher_id", "subject"})

var StanNumberOfSentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "stan_output",
	Name:      "number_of_written_stan_bytes_total",
	Help:      "Number of bytes written by gnmic stan output",
}, []string{"publisher_id", "subject"})

var StanNumberOfFailSendMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "stan_output",
	Name:      "number_of_stan_msgs_sent_fail_total",
	Help:      "Number of failed msgs sent by gnmic stan output",
}, []string{"publisher_id", "reason"})

var StanSendDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "stan_output",
	Name:      "msg_send_duration_ns",
	Help:      "gnmic stan output send duration in ns",
}, []string{"publisher_id"})

func initMetrics() {
	StanNumberOfSentMsgs.WithLabelValues("", "").Add(0)
	StanNumberOfSentBytes.WithLabelValues("", "").Add(0)
	StanNumberOfFailSendMsgs.WithLabelValues("", "").Add(0)
	StanSendDuration.WithLabelValues("").Set(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(StanNumberOfSentMsgs); err != nil {
		return err
	}
	if err = reg.Register(StanNumberOfSentBytes); err != nil {
		return err
	}
	if err = reg.Register(StanNumberOfFailSendMsgs); err != nil {
		return err
	}
	if err = reg.Register(StanSendDuration); err != nil {
		return err
	}
	return nil
}
