// © 2023 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"sync"

	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace                   = "gnmic"
	subsystem                   = "prometheus_write_output"
	inputDropReasonBufferFull   = "buffer_full"
	inputDropReasonBufferResize = "buffer_resize"
)

var registerMetricsOnce sync.Once

var prometheusWriteNumberOfSentMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "number_of_prometheus_write_msgs_sent_success_total",
	Help:      "Number of msgs successfully sent by gnmic prometheus_write output",
}, []string{"name"})

var prometheusWriteNumberOfFailSendMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "number_of_prometheus_write_msgs_sent_fail_total",
	Help:      "Number of failed msgs sent by gnmic prometheus_write output",
}, []string{"name", "reason"})

var prometheusWriteSendDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "msg_send_duration_ns",
	Help:      "gnmic prometheus_write output send duration in ns",
}, []string{"name"})

var prometheusWriteNumberOfSentMetadataMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "number_of_prometheus_write_metadata_msgs_sent_success_total",
	Help:      "Number of metadata msgs successfully sent by gnmic prometheus_write output",
}, []string{"name"})

var prometheusWriteNumberOfFailSendMetadataMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "number_of_prometheus_write_metadata_msgs_sent_fail_total",
	Help:      "Number of failed metadata msgs sent by gnmic prometheus_write output",
}, []string{"name", "reason"})

var prometheusWriteMetadataSendDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "metadata_msg_send_duration_ns",
	Help:      "gnmic prometheus_write output metadata send duration in ns",
}, []string{"name"})

var prometheusWriteInputQueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "input_queue_depth",
	Help:      "Number of gNMI messages waiting for prometheus_write workers",
}, []string{"name"})

var prometheusWriteInputQueueCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "input_queue_capacity",
	Help:      "Maximum number of gNMI messages waiting for prometheus_write workers",
}, []string{"name"})

var prometheusWriteInputMessagesDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "input_messages_dropped_total",
	Help:      "Number of gNMI messages dropped before prometheus_write processing",
}, []string{"name", "reason"})

func initMetrics(name string) {
	// data msgs metrics
	prometheusWriteNumberOfSentMsgs.WithLabelValues(name).Add(0)
	prometheusWriteNumberOfFailSendMsgs.WithLabelValues(name, "").Add(0)
	prometheusWriteSendDuration.WithLabelValues(name).Set(0)
	// metadata msgs metrics
	prometheusWriteNumberOfSentMetadataMsgs.WithLabelValues(name).Add(0)
	prometheusWriteNumberOfFailSendMetadataMsgs.WithLabelValues(name, "").Add(0)
	prometheusWriteMetadataSendDuration.WithLabelValues(name).Set(0)
	prometheusWriteInputQueueDepth.WithLabelValues(name).Set(0)
	prometheusWriteInputQueueCapacity.WithLabelValues(name).Set(0)
	prometheusWriteInputMessagesDropped.WithLabelValues(name, inputDropReasonBufferFull).Add(0)
	prometheusWriteInputMessagesDropped.WithLabelValues(name, inputDropReasonBufferResize).Add(0)
}

func (p *promWriteOutput) registerMetrics() error {
	cfg := p.cfg.Load()
	if cfg == nil {
		return nil
	}
	if !cfg.EnableMetrics {
		return nil
	}
	if p.reg == nil {
		return nil
	}
	var err error
	registerMetricsOnce.Do(func() {
		if err = p.reg.Register(prometheusWriteNumberOfSentMsgs); err != nil {
			p.logger.Error("failed to register metric", "err", err)
			return
		}
		if err = p.reg.Register(prometheusWriteNumberOfFailSendMsgs); err != nil {
			p.logger.Error("failed to register metric", "err", err)
			return
		}
		if err = p.reg.Register(prometheusWriteSendDuration); err != nil {
			p.logger.Error("failed to register metric", "err", err)
			return
		}
		if err = p.reg.Register(prometheusWriteNumberOfSentMetadataMsgs); err != nil {
			p.logger.Error("failed to register metric", "err", err)
			return
		}
		if err = p.reg.Register(prometheusWriteNumberOfFailSendMetadataMsgs); err != nil {
			p.logger.Error("failed to register metric", "err", err)
			return
		}
		if err = p.reg.Register(prometheusWriteMetadataSendDuration); err != nil {
			p.logger.Error("failed to register metric", "err", err)
			return
		}
		if err = p.reg.Register(prometheusWriteInputQueueDepth); err != nil {
			p.logger.Error("failed to register metric", "err", err)
			return
		}
		if err = p.reg.Register(prometheusWriteInputQueueCapacity); err != nil {
			p.logger.Error("failed to register metric", "err", err)
			return
		}
		if err = p.reg.Register(prometheusWriteInputMessagesDropped); err != nil {
			p.logger.Error("failed to register metric", "err", err)
			return
		}
	})
	initMetrics(cfg.Name)
	return err
}

func setInputQueueMetrics(name string, ch chan *outputs.ProtoMsg) {
	prometheusWriteInputQueueDepth.WithLabelValues(name).Set(float64(len(ch)))
	prometheusWriteInputQueueCapacity.WithLabelValues(name).Set(float64(cap(ch)))
}
