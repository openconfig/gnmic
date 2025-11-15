// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package influxdb_output

import (
	"context"
	"fmt"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
)

func (i *influxDBOutput) initCache(ctx context.Context, name string) error {
	var err error
	cfg := i.cfg.Load()
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	i.gnmiCache, err = cache.New(cfg.CacheConfig, cache.WithLogger(i.logger))
	if err != nil {
		return err
	}
	i.cacheTicker = time.NewTicker(cfg.CacheFlushTimer)
	i.done = make(chan struct{})
	go i.runCache(ctx, name)
	return nil
}

func (i *influxDBOutput) stopCache() {
	i.cacheTicker.Stop()
	close(i.done)
	i.gnmiCache.Stop()
}

func (i *influxDBOutput) runCache(ctx context.Context, name string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-i.done:
			return
		case <-i.cacheTicker.C:
			cfg := i.cfg.Load()
			if cfg == nil {
				continue
			}
			if cfg.Debug {
				i.logger.Printf("cache timer tick")
			}
			i.readCache(ctx)
		}
	}
}

func (i *influxDBOutput) readCache(ctx context.Context) {
	notifications, err := i.gnmiCache.ReadAll()
	if err != nil {
		i.logger.Printf("failed to read from cache: %v", err)
		return
	}
	cfg := i.cfg.Load()
	dc := i.dynCfg.Load()

	if cfg == nil || dc == nil {
		return
	}

	if cfg.Debug {
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

	for _, proc := range dc.evps {
		events = proc.Apply(events...)
	}

	resetChan := i.reset.Load()
	if resetChan == nil {
		return
	}
	for _, ev := range events {
		select {
		case <-ctx.Done():
			return
		case <-*resetChan:
			return
		case i.eventChan <- ev:
		}
	}
}

func cacheCfgEqual(a, b *cache.Config) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Compare the fields you actually use; example:
	return a.Type == b.Type &&
		a.Expiration == b.Expiration &&
		a.Debug == b.Debug &&
		a.Address == b.Address &&
		a.Timeout == b.Timeout &&
		a.Username == b.Username &&
		a.Password == b.Password &&
		a.MaxBytes == b.MaxBytes &&
		a.MaxMsgsPerSubscription == b.MaxMsgsPerSubscription &&
		a.FetchBatchSize == b.FetchBatchSize &&
		a.FetchWaitTime == b.FetchWaitTime
}
