// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package snmpoutput

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerMetricsOnce sync.Once

var snmpNumberOfSentTraps = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "snmp_output",
	Name:      "number_of_snmp_traps_sent_total",
	Help:      "Number of SNMP trap sent by gnmic SNMP output",
}, []string{"name", "trap_index"})

var snmpNumberOfTrapSendFailureTraps = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "snmp_output",
	Name:      "number_of_snmp_trap_sent_fail_total",
	Help:      "Number of SNMP trap sending failures",
}, []string{"name", "trap_index", "reason"})

var snmpNumberOfFailedTrapGeneration = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "snmp_output",
	Name:      "number_of_snmp_trap_failed_generation",
	Help:      "Number of failed trap generation",
}, []string{"name", "trap_index", "reason"})

var snmpTrapGenerationDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "snmp_output",
	Name:      "snmp_trap_generation_duration_ns",
	Help:      "SNMP trap generation duration in ns",
}, []string{"name", "trap_index"})

func (s *snmpOutput) initMetrics() {
	snmpNumberOfSentTraps.WithLabelValues(s.name, "0").Add(0)
	snmpNumberOfTrapSendFailureTraps.WithLabelValues(s.name, "0", "").Add(0)
	snmpNumberOfFailedTrapGeneration.WithLabelValues(s.name, "0", "").Add(0)
	snmpTrapGenerationDuration.WithLabelValues(s.name, "0").Set(0)
}

func (s *snmpOutput) registerMetrics() error {
	cfg := s.cfg.Load()
	if cfg == nil {
		return nil
	}
	if !cfg.EnableMetrics {
		return nil
	}
	if s.reg == nil {
		s.logger.Printf("ERR: metrics enabled but main registry is not initialized, enable main metrics under `api-server`")
		return nil
	}

	var err error
	registerMetricsOnce.Do(func() {
		if err = s.reg.Register(snmpNumberOfSentTraps); err != nil {
			return
		}
		if err = s.reg.Register(snmpNumberOfTrapSendFailureTraps); err != nil {
			return
		}
		if err = s.reg.Register(snmpNumberOfFailedTrapGeneration); err != nil {
			return
		}
		if err = s.reg.Register(snmpTrapGenerationDuration); err != nil {
			return
		}
	})
	s.initMetrics()
	return err
}
