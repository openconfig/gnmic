// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package jetstream_output

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerMetricsOnce sync.Once

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

func (n *jetstreamOutput) initMetrics() {
	currCfg := n.cfg.Load()
	if currCfg == nil {
		return
	}
	jetStreamNumberOfSentMsgs.WithLabelValues(currCfg.Name, "").Add(0)
	jetStreamNumberOfSentBytes.WithLabelValues(currCfg.Name, "").Add(0)
	jetStreamNumberOfFailSendMsgs.WithLabelValues(currCfg.Name, "").Add(0)
	jetStreamSendDuration.WithLabelValues(currCfg.Name).Set(0)
}

func (n *jetstreamOutput) registerMetrics() error {
	currCfg := n.cfg.Load()
	if currCfg == nil {
		return nil
	}
	if !currCfg.EnableMetrics {
		return nil
	}
	if n.reg == nil {
		n.logger.Printf("ERR: metrics enabled but main registry is not initialized, enable main metrics under `api-server`")
		return nil
	}

	var err error
	registerMetricsOnce.Do(func() {
		if err = n.reg.Register(jetStreamNumberOfSentMsgs); err != nil {
			return
		}
		if err = n.reg.Register(jetStreamNumberOfSentBytes); err != nil {
			return
		}
		if err = n.reg.Register(jetStreamNumberOfFailSendMsgs); err != nil {
			return
		}
		if err = n.reg.Register(jetStreamSendDuration); err != nil {
			return
		}
	})
	n.initMetrics()
	return err
}
