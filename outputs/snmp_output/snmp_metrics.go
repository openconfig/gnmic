// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package snmpoutput

import "github.com/prometheus/client_golang/prometheus"

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

func (s *snmpOutput) registerMetrics(reg *prometheus.Registry) error {
	s.initMetrics()
	var err error
	if err = reg.Register(snmpNumberOfSentTraps); err != nil {
		return err
	}
	if err = reg.Register(snmpNumberOfTrapSendFailureTraps); err != nil {
		return err
	}
	if err = reg.Register(snmpNumberOfFailedTrapGeneration); err != nil {
		return err
	}
	if err = reg.Register(snmpTrapGenerationDuration); err != nil {
		return err
	}
	return nil
}
