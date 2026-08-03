// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package tcp_output

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

var tcpOutputErrors = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "gnmic",
		Subsystem: "tcp_output",
		Name:      "errors_total",
		Help:      "Number of TCP output delivery errors",
	},
	[]string{"name", "reason"},
)

var tcpOutputDroppedMessages = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "gnmic",
		Subsystem: "tcp_output",
		Name:      "dropped_messages_total",
		Help:      "Number of TCP output messages dropped",
	},
	[]string{"name", "reason"},
)

func registerTCPCollector(
	reg *prometheus.Registry,
	collector prometheus.Collector,
) error {
	if err := reg.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			return nil
		}
		return err
	}
	return nil
}

func (t *tcpOutput) registerMetrics(cfg *config) error {
	if cfg == nil || !cfg.EnableMetrics {
		return nil
	}
	if t.reg == nil {
		t.logger.Error(
			"metrics enabled but main registry is not initialized, enable main metrics under `api-server`",
		)
		return nil
	}

	if err := registerTCPCollector(t.reg, tcpOutputErrors); err != nil {
		return err
	}
	if err := registerTCPCollector(t.reg, tcpOutputDroppedMessages); err != nil {
		return err
	}

	tcpOutputErrors.WithLabelValues(t.name, "dial").Add(0)
	tcpOutputErrors.WithLabelValues(t.name, "write").Add(0)
	tcpOutputDroppedMessages.WithLabelValues(t.name, "max_retries").Add(0)
	return nil
}
