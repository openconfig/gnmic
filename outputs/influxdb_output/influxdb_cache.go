package influxdb_output

import (
	"context"
	"time"

	"github.com/karimra/gnmic/cache"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/proto/gnmi"
)

func (i *InfluxDBOutput) initCache(ctx context.Context, name string) error {
	var err error
	i.gnmiCache, err = cache.New(i.Cfg.CacheConfig, cache.WithLogger(i.logger))
	if err != nil {
		return err
	}
	i.cacheTicker = time.NewTicker(i.Cfg.CacheFlushTimer)
	i.done = make(chan struct{})
	go i.runCache(ctx, name)
	return nil
}

func (i *InfluxDBOutput) stopCache() {
	i.cacheTicker.Stop()
	close(i.done)
	i.gnmiCache.Stop()
}

func (i *InfluxDBOutput) runCache(ctx context.Context, name string) {
	for {
		select {
		case <-i.done:
			return
		case <-i.cacheTicker.C:
			if i.Cfg.Debug {
				i.logger.Printf("cache timer tick")
			}
			i.readCache(ctx, name)
		}
	}
}

func (i *InfluxDBOutput) readCache(ctx context.Context, name string) {
	notifications, err := i.gnmiCache.Read()
	if err != nil {
		i.logger.Printf("failed to read from cache: %v", err)
		return
	}
	if i.Cfg.Debug {
		i.logger.Printf("read notifications: %+v", notifications)
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
				i.logger.Printf("failed to convert gNMI notifications to events: %v", err)
				return
			}
			events = append(events, ievents...)
		}
	}

	// apply processors if any
	for _, proc := range i.evps {
		events = proc.Apply(events...)
	}

	for _, ev := range events {
		select {
		case <-i.reset:
			return
		case i.eventChan <- ev:
		}
	}
}
