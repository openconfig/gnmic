// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package kafka_output

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerMetricsOnce sync.Once

var kafkaNumberOfSentMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "number_of_kafka_msgs_sent_success_total",
	Help:      "Number of msgs successfully sent by gnmic kafka output",
}, []string{"name", "producer_id"})

var kafkaNumberOfSentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "number_of_written_kafka_bytes_total",
	Help:      "Number of bytes written by gnmic kafka output",
}, []string{"name", "producer_id"})

var kafkaNumberOfFailSendMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "number_of_kafka_msgs_sent_fail_total",
	Help:      "Number of failed msgs sent by gnmic kafka output",
}, []string{"name", "producer_id", "reason"})

var kafkaSendDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "msg_send_duration_ns",
	Help:      "gnmic kafka output send duration in ns",
}, []string{"name", "producer_id"})

func (k *kafkaOutput) initMetrics(name string) {
	kafkaNumberOfSentMsgs.WithLabelValues(name, "").Add(0)
	kafkaNumberOfSentBytes.WithLabelValues(name, "").Add(0)
	kafkaNumberOfFailSendMsgs.WithLabelValues(name, "", "").Add(0)
	kafkaSendDuration.WithLabelValues(name, "").Set(0)
}

func (k *kafkaOutput) registerMetrics() error {
	cfg := k.cfg.Load()
	if cfg == nil {
		return nil
	}
	if !cfg.EnableMetrics {
		return nil
	}
	if k.reg == nil {
		k.logger.Printf("ERR: metrics enabled but main registry is not initialized, enable main metrics under `api-server`")
		return nil
	}
	var err error
	registerMetricsOnce.Do(func() {
		if err = k.reg.Register(kafkaNumberOfSentMsgs); err != nil {
			return
		}
		if err = k.reg.Register(kafkaNumberOfSentBytes); err != nil {
			return
		}
		if err = k.reg.Register(kafkaNumberOfFailSendMsgs); err != nil {
			return
		}
		if err = k.reg.Register(kafkaSendDuration); err != nil {
			return
		}
	})
	k.initMetrics(cfg.Name)
	return err
}
