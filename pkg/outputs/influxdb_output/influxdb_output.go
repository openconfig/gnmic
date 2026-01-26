// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package influxdb_output

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"math"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"text/template"
	"time"

	"google.golang.org/protobuf/proto"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	gutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
)

const (
	defaultURL             = "http://localhost:8086"
	defaultBatchSize       = 1000
	defaultFlushTimer      = 10 * time.Second
	minHealthCheckPeriod   = 30 * time.Second
	defaultCacheFlushTimer = 5 * time.Second

	numWorkers     = 1
	loggingPrefix  = "[influxdb_output:%s] "
	deleteTagValue = "true"
)

func init() {
	outputs.Register("influxdb", func() outputs.Output {
		return &influxDBOutput{}
	})
}

type influxDBOutput struct {
	outputs.BaseOutput

	cfg    *atomic.Pointer[Config]
	dynCfg *atomic.Pointer[dynConfig]
	client *atomic.Pointer[influxdb2.Client]

	logger    *log.Logger
	cancelFn  context.CancelFunc
	eventChan chan *formatters.EventMsg

	reset    *atomic.Pointer[chan struct{}]
	startSig *atomic.Pointer[chan struct{}]
	wasUP    atomic.Bool

	dbVersion atomic.Value // stores string

	gnmiCache   cache.Cache
	cacheTicker *time.Ticker
	done        chan struct{}

	store        store.Store[any]
	healthCancel context.CancelFunc
}

func (i *influxDBOutput) init() {
	i.cfg = new(atomic.Pointer[Config])
	i.dynCfg = new(atomic.Pointer[dynConfig])
	i.client = new(atomic.Pointer[influxdb2.Client])
	i.eventChan = make(chan *formatters.EventMsg)
	i.reset = new(atomic.Pointer[chan struct{}])
	i.startSig = new(atomic.Pointer[chan struct{}])
	i.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
}

type dynConfig struct {
	targetTpl *template.Template
	evps      []formatters.EventProcessor
}

type Config struct {
	Name               string           `mapstructure:"name,omitempty"`
	URL                string           `mapstructure:"url,omitempty"`
	Org                string           `mapstructure:"org,omitempty"`
	Bucket             string           `mapstructure:"bucket,omitempty"`
	Token              string           `mapstructure:"token,omitempty"`
	BatchSize          uint             `mapstructure:"batch-size,omitempty"`
	FlushTimer         time.Duration    `mapstructure:"flush-timer,omitempty"`
	UseGzip            bool             `mapstructure:"use-gzip,omitempty"`
	EnableTLS          bool             `mapstructure:"enable-tls,omitempty"`
	TLS                *types.TLSConfig `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	HealthCheckPeriod  time.Duration    `mapstructure:"health-check-period,omitempty"`
	Debug              bool             `mapstructure:"debug,omitempty"`
	AddTarget          string           `mapstructure:"add-target,omitempty"`
	TargetTemplate     string           `mapstructure:"target-template,omitempty"`
	EventProcessors    []string         `mapstructure:"event-processors,omitempty"`
	EnableMetrics      bool             `mapstructure:"enable-metrics,omitempty"`
	OverrideTimestamps bool             `mapstructure:"override-timestamps,omitempty"`
	TimestampPrecision string           `mapstructure:"timestamp-precision,omitempty"`
	CacheConfig        *cache.Config    `mapstructure:"cache,omitempty"`
	CacheFlushTimer    time.Duration    `mapstructure:"cache-flush-timer,omitempty"`
	DeleteTag          string           `mapstructure:"delete-tag,omitempty"`
}

func (k *influxDBOutput) String() string {
	cfg := k.cfg.Load()
	if cfg == nil {
		return ""
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (i *influxDBOutput) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(i.store)
	if err != nil {
		return nil, err
	}

	evps, err := formatters.MakeEventProcessors(
		logger,
		eventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return nil, err
	}
	return evps, nil
}

func (i *influxDBOutput) setLogger(logger *log.Logger) {
	if logger != nil && i.logger != nil {
		i.logger.SetOutput(logger.Writer())
		i.logger.SetFlags(logger.Flags())
	}
}

func (i *influxDBOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	i.init() // init struct fields
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	i.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	i.store = options.Store

	if newCfg.Name == "" {
		newCfg.Name = name
	}

	// apply logger
	i.setLogger(options.Logger)

	// set defaults
	i.setDefaultsFor(newCfg)

	if _, err := url.Parse(newCfg.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	// store config
	i.cfg.Store(newCfg)

	// build dynamic config
	dc := new(dynConfig)

	// initialize event processors
	dc.evps, err = i.buildEventProcessors(options.Logger, newCfg.EventProcessors)
	if err != nil {
		return err
	}

	// initialize template
	if newCfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	i.dynCfg.Store(dc)

	// initialize cache
	if newCfg.CacheConfig != nil {
		err = i.initCache(ctx, name)
		if err != nil {
			return err
		}
	}

	// initialize reset and startSig channels
	resetChan := make(chan struct{})
	i.reset.Store(&resetChan)
	startSigChan := make(chan struct{})
	i.startSig.Store(&startSigChan)

	ctx, i.cancelFn = context.WithCancel(ctx)

	influxOpts, err := clientOptsFor(newCfg)
	if err != nil {
		return err
	}
	// initialize influxdb client
CRCLIENT:
	if ctx.Err() != nil {
		return ctx.Err()
	}
	newClient := influxdb2.NewClientWithOptions(newCfg.URL, newCfg.Token, influxOpts)
	i.client.Store(&newClient)

	// start influx health check
	if newCfg.HealthCheckPeriod > 0 {
		err = i.health(ctx)
		if err != nil {
			i.logger.Printf("failed to check influxdb health: %v", err)
			time.Sleep(2 * time.Second)
			goto CRCLIENT
		}
		hcCtx, hcCancel := context.WithCancel(ctx)
		i.healthCancel = hcCancel
		go i.healthCheck(hcCtx)
	}

	i.wasUP.Store(true)
	i.logger.Printf("initialized influxdb client: %s", i.String())

	for k := 0; k < numWorkers; k++ {
		go i.worker(ctx, k)
	}
	go func() {
		<-ctx.Done()
		i.Close()
	}()
	return nil
}

func (i *influxDBOutput) setDefaultsFor(c *Config) {
	if c.URL == "" {
		c.URL = defaultURL
	}
	if c.BatchSize == 0 {
		c.BatchSize = defaultBatchSize
	}
	if c.FlushTimer == 0 {
		c.FlushTimer = defaultFlushTimer
	}
	if c.HealthCheckPeriod != 0 && c.HealthCheckPeriod < minHealthCheckPeriod {
		c.HealthCheckPeriod = minHealthCheckPeriod
	}
	if c.CacheConfig != nil {
		if c.CacheFlushTimer == 0 {
			c.CacheFlushTimer = defaultCacheFlushTimer
		}
	}
}

// Build influx options from an arbitrary config (no side effects on i.cfg)
func clientOptsFor(c *Config) (*influxdb2.Options, error) {
	iopts := influxdb2.DefaultOptions().
		SetUseGZip(c.UseGzip).
		SetBatchSize(c.BatchSize).
		SetFlushInterval(uint(c.FlushTimer.Milliseconds()))

	// TLS from explicit TLS config
	if c.TLS != nil {
		tlsConfig, err := utils.NewTLSConfig(
			c.TLS.CaFile,
			c.TLS.CertFile,
			c.TLS.KeyFile,
			"",
			c.TLS.SkipVerify,
			false,
		)
		if err != nil {
			return nil, err
		}
		iopts.SetTLSConfig(tlsConfig)
	}

	// Legacy "EnableTLS" flag (insecure)
	if c.EnableTLS {
		iopts.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
	}

	switch c.TimestampPrecision {
	case "s":
		iopts.SetPrecision(time.Second)
	case "ms":
		iopts.SetPrecision(time.Millisecond)
	case "us":
		iopts.SetPrecision(time.Microsecond)
	}

	if c.Debug {
		iopts.SetLogLevel(3)
	}
	return iopts, nil
}

func (i *influxDBOutput) Validate(cfg map[string]any) error {
	ncfg := new(Config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}

	if _, err := url.Parse(ncfg.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	_, err = gtemplate.CreateTemplate("target-template", ncfg.TargetTemplate)
	if err != nil {
		return err
	}
	return nil
}

func (i *influxDBOutput) Update(ctx context.Context, cfg map[string]any) error {
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}

	currCfg := i.cfg.Load()
	if newCfg.Name == "" && currCfg != nil {
		newCfg.Name = currCfg.Name
	}

	i.setDefaultsFor(newCfg)

	// check if event processors changed
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0

	// rebuild dynamic config
	dc := new(dynConfig)

	// rebuild templates
	if newCfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		t, err := gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = t.Funcs(outputs.TemplateFuncs)
	} else {
		dc.targetTpl = outputs.DefaultTargetTemplate
	}

	// rebuild event processors if needed
	prevDC := i.dynCfg.Load()
	if rebuildProcessors {
		dc.evps, err = i.buildEventProcessors(i.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}

	// store new dynamic config
	i.dynCfg.Store(dc)
	// store new config
	i.cfg.Store(newCfg)
	// check if client needs rebuild
	needsClientRebuild := clientNeedsRebuild(currCfg, newCfg)

	if needsClientRebuild {
		// rebuild influxdb client options
		iopts, err := clientOptsFor(newCfg)
		if err != nil {
			return err
		}

		// rebuild influxdb client
		newClient := influxdb2.NewClientWithOptions(newCfg.URL, newCfg.Token, iopts)

		// health check if enabled
		if newCfg.HealthCheckPeriod > 0 {
			if _, err := newClient.Health(ctx); err != nil {
				// do not return error, continue
				i.logger.Printf("update: influx health probe failed (continuing): %v", err)
			}
		}

		// swap client
		oldClientPtr := i.client.Swap(&newClient)
		oldClient := *oldClientPtr

		// close old client
		if oldClient != nil {
			oldClient.Close()
		}

		// signal workers to rebuild their write APIs
		oldReset := i.reset.Load()
		newResetChan := make(chan struct{})
		i.reset.Store(&newResetChan)
		close(*oldReset)
	}

	// cache toggle
	oldHadCache := currCfg != nil && currCfg.CacheConfig != nil
	newHasCache := newCfg.CacheConfig != nil
	switch {
	case oldHadCache && !newHasCache:
		// stop old cache if present
		i.stopCache()
	case !oldHadCache && newHasCache:
		// init new cache if requested
		if err := i.initCache(ctx, newCfg.Name); err != nil {
			return err
		}
	case oldHadCache && newHasCache:
		// check if cache config changed
		sameCacheConfig := cacheCfgEqual(currCfg.CacheConfig, newCfg.CacheConfig)
		if sameCacheConfig {
			if currCfg.CacheFlushTimer != newCfg.CacheFlushTimer {
				// change flush timer
				if i.cacheTicker != nil {
					i.cacheTicker.Stop()
				}
				i.cacheTicker = time.NewTicker(newCfg.CacheFlushTimer)
			}
		} else {
			// cache config changed, stop old cache and init new cache
			i.stopCache()
			if err := i.initCache(ctx, newCfg.Name); err != nil {
				return err
			}
		}
	}

	// handle health check changes
	oldPeriod := time.Duration(0)
	if currCfg != nil {
		oldPeriod = currCfg.HealthCheckPeriod
	}
	newPeriod := newCfg.HealthCheckPeriod
	periodChanged := oldPeriod != newPeriod
	enabledChanged := (oldPeriod == 0) != (newPeriod == 0)

	if enabledChanged || periodChanged {
		if i.healthCancel != nil {
			i.healthCancel()
			i.healthCancel = nil
		}
		if newPeriod > 0 {
			_ = i.health(ctx)
			hcCtx, hcCancel := context.WithCancel(ctx)
			i.healthCancel = hcCancel
			go i.healthCheck(hcCtx)
		}
	}

	i.logger.Printf("updated influxdb output: %s", i.String())
	return nil
}

func (i *influxDBOutput) UpdateProcessor(name string, pcfg map[string]any) error {
	cfg := i.cfg.Load()
	dc := i.dynCfg.Load()

	newEvps, changed, err := outputs.UpdateProcessorInSlice(
		i.logger,
		i.store,
		cfg.EventProcessors,
		dc.evps,
		name,
		pcfg,
	)
	if err != nil {
		return err
	}
	if changed {
		newDC := *dc
		newDC.evps = newEvps
		i.dynCfg.Store(&newDC)
		i.logger.Printf("updated event processor %s", name)
	}
	return nil
}

func (i *influxDBOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	cfg := i.cfg.Load()
	dc := i.dynCfg.Load()
	resetChan := i.reset.Load()

	if cfg == nil || dc == nil || resetChan == nil {
		return
	}

	var err error
	rsp, err = outputs.AddSubscriptionTarget(rsp, meta, cfg.AddTarget, dc.targetTpl)
	if err != nil {
		i.logger.Printf("failed to add target to the response: %v", err)
	}

	switch rsp := rsp.(type) {
	case *gnmi.SubscribeResponse:
		measName := "default"
		if subName, ok := meta["subscription-name"]; ok {
			measName = subName
		}
		if i.gnmiCache != nil {
			i.gnmiCache.Write(ctx, measName, rsp)
			return
		}
		events, err := formatters.ResponseToEventMsgs(measName, rsp, meta, dc.evps...)
		if err != nil {
			i.logger.Printf("failed to convert message to event: %v", err)
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
}

func (i *influxDBOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	dc := i.dynCfg.Load()
	resetChan := i.reset.Load()

	if dc == nil || resetChan == nil {
		return
	}

	select {
	case <-ctx.Done():
		return
	case <-*resetChan:
		return
	default:
		var evs = []*formatters.EventMsg{ev}
		for _, proc := range dc.evps {
			evs = proc.Apply(evs...)
		}
		for _, pev := range evs {
			i.eventChan <- pev
		}
	}
}

func (i *influxDBOutput) Close() error {
	i.logger.Printf("closing client...")

	cfg := i.cfg.Load()
	if cfg != nil && cfg.CacheConfig != nil {
		i.stopCache()
	}
	if i.healthCancel != nil {
		i.healthCancel()
		i.healthCancel = nil
	}
	i.cancelFn()

	clientPtr := i.client.Load()
	if *clientPtr != nil {
		(*clientPtr).Close()
	}
	reset := i.reset.Load()
	if reset != nil {
		select {
		case <-*reset:
		default:
			close(*reset) // unblock Write() and WriteEvent()
		}
	}
	i.logger.Printf("closed.")
	return nil
}

func (i *influxDBOutput) healthCheck(ctx context.Context) {
	cfg := i.cfg.Load()
	if cfg == nil {
		return
	}

	ticker := time.NewTicker(cfg.HealthCheckPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			i.health(ctx)
		}
	}
}

func (i *influxDBOutput) health(ctx context.Context) error {
	clientPtr := i.client.Load()
	if clientPtr == nil || *clientPtr == nil {
		return fmt.Errorf("client not initialized")
	}

	res, err := (*clientPtr).Health(ctx)
	if err != nil {
		i.logger.Printf("failed health check: %v", err)
		if i.wasUP.Load() {
			oldReset := i.reset.Load()
			newResetChan := make(chan struct{})
			i.reset.Store(&newResetChan)
			close(*oldReset)
		}
		return err
	}

	if res != nil {
		if res.Version != nil {
			i.dbVersion.Store(*res.Version)
		}
		b, err := json.Marshal(res)
		if err != nil {
			i.logger.Printf("failed to marshal health check result: %v", err)
			i.logger.Printf("health check result: %+v", res)
			if i.wasUP.Load() {
				oldReset := i.reset.Load()
				newResetChan := make(chan struct{})
				i.reset.Store(&newResetChan)
				close(*oldReset)
			}
			return err
		}
		i.wasUP.Store(true)
		oldStartSig := i.startSig.Load()
		newStartSigChan := make(chan struct{})
		i.startSig.Store(&newStartSigChan)
		close(*oldStartSig)
		i.logger.Printf("health check result: %s", string(b))
		return nil
	}

	i.wasUP.Store(true)
	oldStartSig := i.startSig.Load()
	newStartSigChan := make(chan struct{})
	i.startSig.Store(&newStartSigChan)
	close(*oldStartSig)
	i.logger.Print("health check result is nil")
	return nil
}

func (i *influxDBOutput) worker(ctx context.Context, idx int) {
	firstStart := true
START:
	if ctx.Err() != nil {
		i.logger.Printf("worker-%d err=%v", idx, ctx.Err())
		return
	}

	cfg := i.cfg.Load()
	if cfg == nil {
		i.logger.Printf("worker-%d: config not initialized", idx)
		return
	}

	if !firstStart && cfg.HealthCheckPeriod > 0 {
		i.logger.Printf("worker-%d waiting for client recovery", idx)
		startSigChan := i.startSig.Load()
		if startSigChan != nil {
			<-*startSigChan
		}
	}

	i.logger.Printf("starting worker-%d", idx)

	clientPtr := i.client.Load()
	if clientPtr == nil || *clientPtr == nil {
		i.logger.Printf("worker-%d: client not initialized", idx)
		return
	}
	client := *clientPtr

	resetChan := i.reset.Load()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				i.logger.Printf("worker-%d err=%v", idx, ctx.Err())
			}
			i.logger.Printf("worker-%d terminating...", idx)
			return
		case ev := <-i.eventChan:
			// Reload config for each event to get fresh values
			cfg := i.cfg.Load()
			if cfg == nil {
				continue
			}

			if len(ev.Values) == 0 && len(ev.Deletes) == 0 {
				continue
			}
			if len(ev.Values) == 0 && cfg.DeleteTag == "" {
				continue
			}

			for n, v := range ev.Values {
				switch v := v.(type) {
				//lint:ignore SA1019 still need DecimalVal for backward compatibility
				case *gnmi.Decimal64:
					ev.Values[n] = float64(v.Digits) / math.Pow10(int(v.Precision))
				}
			}

			if ev.Timestamp == 0 || cfg.OverrideTimestamps {
				ev.Timestamp = time.Now().UnixNano()
			}

			if subscriptionName, ok := ev.Tags["subscription-name"]; ok {
				ev.Name = subscriptionName
				delete(ev.Tags, "subscription-name")
			}

			if len(ev.Values) > 0 {
				i.convertUints(ev)
				client.WriteAPI(cfg.Org, cfg.Bucket).
					WritePoint(influxdb2.NewPoint(ev.Name, ev.Tags, ev.Values, time.Unix(0, ev.Timestamp)))
			}

			if len(ev.Deletes) > 0 && cfg.DeleteTag != "" {
				tags := make(map[string]string, len(ev.Tags))
				maps.Copy(tags, ev.Tags)
				tags[cfg.DeleteTag] = deleteTagValue
				values := make(map[string]any, len(ev.Deletes))
				for _, del := range ev.Deletes {
					values[del] = ""
				}
				client.WriteAPI(cfg.Org, cfg.Bucket).
					WritePoint(influxdb2.NewPoint(ev.Name, tags, values, time.Unix(0, ev.Timestamp)))
			}
		case <-*resetChan:
			firstStart = false
			i.logger.Printf("resetting worker-%d...", idx)
			goto START
		case err := <-client.WriteAPI(cfg.Org, cfg.Bucket).Errors():
			i.logger.Printf("worker-%d write error: %v", idx, err)
		}
	}
}

func (i *influxDBOutput) convertUints(ev *formatters.EventMsg) {
	dbVer := i.dbVersion.Load()
	if dbVer == nil {
		return
	}
	dbVersion, ok := dbVer.(string)
	if !ok || !strings.HasPrefix(dbVersion, "1.8") {
		return
	}

	for k, v := range ev.Values {
		switch v := v.(type) {
		case uint:
			ev.Values[k] = int(v)
		case uint8:
			ev.Values[k] = int(v)
		case uint16:
			ev.Values[k] = int(v)
		case uint32:
			ev.Values[k] = int(v)
		case uint64:
			ev.Values[k] = int(v)
		}
	}
}

func clientNeedsRebuild(old, new *Config) bool {
	if old == nil || new == nil {
		return true
	}
	return old.URL != new.URL ||
		old.Token != new.Token ||
		old.BatchSize != new.BatchSize ||
		old.FlushTimer != new.FlushTimer ||
		old.UseGzip != new.UseGzip ||
		old.EnableTLS != new.EnableTLS ||
		!old.TLS.Equal(new.TLS) ||
		old.TimestampPrecision != new.TimestampPrecision ||
		old.Debug != new.Debug
}
