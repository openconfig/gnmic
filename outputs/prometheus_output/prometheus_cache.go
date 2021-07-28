package prometheus_output

import (
	"sync"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/ctree"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
)

func (p *PrometheusOutput) writeToCache(measName string, rsp *gnmi.SubscribeResponse) {
	var err error
	switch rsp := rsp.GetResponse().(type) {
	case *gnmi.SubscribeResponse_Update:
		target := rsp.Update.GetPrefix().GetTarget()
		if target == "" {
			p.logger.Printf("response missing target")
			return
		}
		p.Lock()
		defer p.Unlock()
		if _, ok := p.caches[measName]; !ok {
			p.caches[measName] = cache.New(nil)
			p.caches[measName].Add(target)
		} else if !p.caches[measName].HasTarget(target) {
			p.caches[measName].Add(target)
			p.logger.Printf("target %q added to the local cache", target)
		}
		if p.Cfg.Debug {
			p.logger.Printf("updating target %q local cache", target)
		}
		err = p.caches[measName].GnmiUpdate(rsp.Update)
		if err != nil {
			p.logger.Printf("failed to update gNMI cache: %v", err)
		}
		return
	}
}

// collectFromCache does the following:
// - runs queries over all the stored caches,
// - retrives the gNMI notifications that are not older that the expiration duration.
// - generates a lit of events from the gNMI notifications list.
// - applies the configured processors on the events list.
// - generates prometheus metrics from the events and sends them to chan<- prometheus.Metric
func (p *PrometheusOutput) collectFromCache(ch chan<- prometheus.Metric) {
	var err error
	evChan := make(chan []*formatters.EventMsg)
	events := make([]*formatters.EventMsg, 0)

	// this go routine will collect all the events
	// from the cache queries
	go func() {
		for evs := range evChan {
			events = append(events, evs...)
		}
	}()

	now := time.Now()
	wg := new(sync.WaitGroup)
	wg.Add(len(p.caches))
	for name, c := range p.caches {
		go func(c *cache.Cache, name string) {
			defer wg.Done()
			err = c.Query("*", []string{},
				func(_ []string, l *ctree.Leaf, v interface{}) error {
					if err != nil {
						return err
					}
					switch notif := v.(type) {
					case *gnmi.Notification:
						if p.Cfg.Expiration > 0 &&
							time.Unix(0, notif.Timestamp).Before(now.Add(time.Duration(-p.Cfg.Expiration))) {
							return nil
						}
						// build events without processors
						events, err := formatters.ResponseToEventMsgs(name, &gnmi.SubscribeResponse{
							Response: &gnmi.SubscribeResponse_Update{Update: notif},
						}, nil)
						if err != nil {
							p.logger.Printf("failed to convert message to event: %v", err)
							return nil
						}
						evChan <- events
					}
					return nil
				})
			if err != nil {
				p.logger.Printf("failed prometheus cache query:%v", err)
				return
			}
		}(c, name)
	}
	wg.Wait()
	close(evChan)
	// apply processors
	for _, proc := range p.evps {
		events = proc.Apply(events...)
	}
	// build prometheus metrics and send
	for _, ev := range events {
		for _, pm := range p.metricsFromEvent(ev, now) {
			ch <- pm
		}
	}
}
