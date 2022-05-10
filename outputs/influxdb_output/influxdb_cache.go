package influxdb_output

import (
	"time"

	"github.com/karimra/gnmic/cache"
)

func (i *InfluxDBOutput) initCache() {
	i.Cfg.GnmiCacheConfig.SetDefaults()
	i.gnmiCache = &cache.GnmiOutputCache{}
	i.gnmiCache.LoadConfig(i.Cfg.GnmiCacheConfig)
	i.gnmiCache.Init(cache.WithLogger(i.logger))
	i.cacheTicker = time.NewTicker(i.Cfg.CacheFlushTimer)
	i.done = make(chan struct{})
	go i.runCache()
}

func (i *InfluxDBOutput) stopCache() {
	i.cacheTicker.Stop()
	close(i.done)
}

func (i *InfluxDBOutput) runCache() {
	for {
		select {
		case <-i.done:
			return
		case <-i.cacheTicker.C:
			if i.Cfg.Debug {
				i.logger.Printf("cache timer tick")
			}
			i.readCache()
		}
	}
}

func (i *InfluxDBOutput) readCache() {
	events := i.gnmiCache.Read()
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
