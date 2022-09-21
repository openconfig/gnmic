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
