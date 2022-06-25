package prometheus_output

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func (p *prometheusOutput) collectFromCache(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	events := p.gnmiCache.ReadEvents()
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
