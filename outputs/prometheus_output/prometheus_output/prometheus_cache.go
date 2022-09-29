// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_output

import (
	"context"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
)

func (p *prometheusOutput) collectFromCache(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	notifications, err := p.gnmiCache.Read()
	if err != nil {
		p.logger.Printf("failed to read from cache: %v", err)
		return
	}
	events := make([]*formatters.EventMsg, 0, len(notifications))
	for subName, notifs := range notifications {
		// build events without processors
		for _, notif := range notifs {
			ievents, err := formatters.ResponseToEventMsgs(subName,
				&gnmi.SubscribeResponse{
					Response: &gnmi.SubscribeResponse_Update{Update: notif},
				},
				outputs.Meta{"subscription-name": subName})
			if err != nil {
				p.logger.Printf("failed to convert gNMI notifications to events: %v", err)
				return
			}
			events = append(events, ievents...)
		}
	}

	for _, proc := range p.evps {
		events = proc.Apply(events...)
	}
	now := time.Now()
	for _, ev := range events {
		for _, pm := range p.metricsFromEvent(ev, now) {
			select {
			case <-ctx.Done():
				p.logger.Printf("collection context terminated: %v", ctx.Err())
				return
			case ch <- pm:
			}
		}
	}
}
