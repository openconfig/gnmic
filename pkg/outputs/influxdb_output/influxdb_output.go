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
	"log/slog"
	"maps"
	"math"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"google.golang.org/protobuf/proto"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/logging"
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
	outputType     = "influxdb"
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

	logger    *slog.Logger
	eventChan chan *formatters.EventMsg

	// rootCtx is the context Init was called with. Each worker generation runs
	// under a context derived from it, so a client rebuild can cancel and wait
	// for the previous generation without tearing down the output.
	rootCtx  context.Context
	cancelFn context.CancelFunc
	// wg tracks the current worker generation. It is replaced together with
	// cancelFn whenever workers are restarted; each worker is handed the wg it
	// was started with so Done() always targets the right generation.
	wg        *sync.WaitGroup
	closeOnce sync.Once

	reset    *atomic.Pointer[chan struct{}]
	startSig *atomic.Pointer[chan struct{}]
	wasUP    atomic.Bool

	dbVersion atomic.Value // stores string

	gnmiCache   cache.Cache
	cacheTicker *time.Ticker
	done        chan struct{}

	store        store.Store[any]
	healthCancel context.CancelFunc
	reg          *prometheus.Registry

	nonFiniteLog nonFiniteLogState
}

func (i *influxDBOutput) init() {
	i.cfg = new(atomic.Pointer[Config])
	i.dynCfg = new(atomic.Pointer[dynConfig])
	i.client = new(atomic.Pointer[influxdb2.Client])
	i.eventChan = make(chan *formatters.EventMsg)
	i.reset = new(atomic.Pointer[chan struct{}])
	i.startSig = new(atomic.Pointer[chan struct{}])
	i.wg = new(sync.WaitGroup)
	i.logger = logging.DiscardLogger()
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

func (c *Config) LogValue() slog.Value {
	return logging.RedactedValue(c)
}

func (k *influxDBOutput) String() string {
	cfg := k.cfg.Load()
	if cfg == nil {
		return ""
	}
	return logging.RedactedJSON(cfg)
}

func (i *influxDBOutput) buildEventProcessors(logger *slog.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
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

func (i *influxDBOutput) setLogger(logger *slog.Logger, name string) {
	i.logger = outputs.BindLogger(logger, outputType, name)
}

func (i *influxDBOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	i.init() // init struct fields
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	i.store = options.Store
	i.reg = options.Registry

	if newCfg.Name == "" {
		newCfg.Name = name
	}

	// apply logger
	i.setLogger(options.Logger, newCfg.Name)

	// set defaults
	i.setDefaultsFor(newCfg)

	if _, err := url.Parse(newCfg.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	// store config
	i.cfg.Store(newCfg)

	if err := i.registerMetrics(); err != nil {
		return err
	}
	i.initMetricLabels()

	// build dynamic config
	dc := new(dynConfig)

	// initialize event processors
	dc.evps, err = i.buildEventProcessors(i.logger, newCfg.EventProcessors)
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

	i.rootCtx = ctx
	ctx, i.cancelFn = context.WithCancel(ctx)

	influxOpts, err := clientOptsFor(newCfg)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// initialize influxdb client. NewClientWithOptions performs no I/O, so it
	// cannot fail because the server is unreachable.
	newClient := influxdb2.NewClientWithOptions(newCfg.URL, newCfg.Token, influxOpts)
	i.client.Store(&newClient)

	// start influx health check
	if newCfg.HealthCheckPeriod > 0 {
		// Probe once so an unreachable server is visible in the logs at
		// startup, but do not block on it: Init previously retried here
		// forever, so gnmic never finished starting while influx was down.
		// Recovery is the health check goroutine's job, matching Update().
		if err := i.health(ctx); err != nil {
			i.logger.Warn("influxdb health probe failed at init (continuing)", "err", err)
		}
		hcCtx, hcCancel := context.WithCancel(ctx)
		i.healthCancel = hcCancel
		go i.healthCheck(hcCtx)
	}

	i.wasUP.Store(true)
	i.logger.Info("initialized influxdb client", slog.Any("config", i.String()))

	i.wg.Add(numWorkers)
	for k := 0; k < numWorkers; k++ {
		go i.worker(ctx, i.wg, k)
	}
	// Watch the root context, not the worker-generation context: Update()
	// cancels the latter to drain the previous worker generation, which must
	// not be mistaken for the output shutting down.
	go func() {
		<-i.rootCtx.Done()
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
				i.logger.Warn("update: influx health probe failed (continuing)", "err", err)
			}
		}

		// Publish the new client before starting the new workers so they pick
		// it up at their START label.
		oldClientPtr := i.client.Swap(&newClient)

		// Restart the workers rather than signalling them through the reset
		// channel. Workers capture the client once and hold it for the whole
		// select loop, so the old client cannot be closed until every worker
		// that captured it has exited -- closing it first is a use-after-close.
		oldWG := i.wg
		oldCancel := i.cancelFn

		newWG := new(sync.WaitGroup)
		i.wg = newWG
		runCtx, cancel := context.WithCancel(i.rootCtx)
		i.cancelFn = cancel

		// Start the new generation first so eventChan always has a consumer and
		// Write() never blocks during the swap.
		newWG.Add(numWorkers)
		for k := 0; k < numWorkers; k++ {
			go i.worker(runCtx, newWG, k)
		}

		// Now drain the old generation, and only then close the client it held.
		if oldCancel != nil {
			oldCancel()
		}
		if oldWG != nil {
			oldWG.Wait()
		}
		if oldClientPtr != nil && *oldClientPtr != nil {
			(*oldClientPtr).Close()
		}
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

	// enable-metrics may have been switched on by this reload, in which case the
	// collector was never registered at Init.
	if err := i.registerMetrics(); err != nil {
		return err
	}
	i.initMetricLabels()

	i.logger.Info("updated influxdb output", slog.Any("config", i.String()))
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
		i.logger.Info("updated event processor", "name", name)
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
		i.logger.Warn("failed to add target to response", "err", err)
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
			i.logger.Warn("failed to convert message to event", "err", err)
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
	i.closeOnce.Do(func() {
		i.logger.Info("closing client")

		cfg := i.cfg.Load()
		if cfg != nil && cfg.CacheConfig != nil {
			i.stopCache()
		}
		if i.healthCancel != nil {
			i.healthCancel()
			i.healthCancel = nil
		}
		if i.cancelFn != nil {
			i.cancelFn()
		}

		reset := i.reset.Load()
		if reset != nil {
			select {
			case <-*reset:
			default:
				close(*reset) // unblock Write() and WriteEvent()
			}
		}

		// Wait for the workers to release the client before closing it.
		if i.wg != nil {
			i.wg.Wait()
		}

		clientPtr := i.client.Load()
		if clientPtr != nil && *clientPtr != nil {
			(*clientPtr).Close()
		}
		i.logger.Info("closed")
	})
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
		i.logger.Warn("failed health check", "err", err)
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
			i.logger.Warn("failed to marshal health check result", "err", err)
			i.logger.Debug("health check result", "result", res)
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
		i.logger.Debug("health check result", "result", string(b))
		return nil
	}

	i.wasUP.Store(true)
	oldStartSig := i.startSig.Load()
	newStartSigChan := make(chan struct{})
	i.startSig.Store(&newStartSigChan)
	close(*oldStartSig)
	i.logger.Debug("health check result is nil")
	return nil
}

func (i *influxDBOutput) worker(ctx context.Context, wg *sync.WaitGroup, idx int) {
	defer wg.Done()
	firstStart := true
START:
	if ctx.Err() != nil {
		i.logger.Warn("worker err", "worker", idx, "err", ctx.Err())
		return
	}

	cfg := i.cfg.Load()
	if cfg == nil {
		i.logger.Warn("worker: config not initialized", "worker", idx)
		return
	}

	if !firstStart && cfg.HealthCheckPeriod > 0 {
		i.logger.Info("worker waiting for client recovery", "worker", idx)
		startSigChan := i.startSig.Load()
		if startSigChan != nil {
			// Must stay cancellable: Close() waits on the worker WaitGroup, so a
			// worker parked here while influx is down would otherwise hang
			// shutdown until the health check recovers.
			select {
			case <-ctx.Done():
				i.logger.Info("worker terminating", "worker", idx)
				return
			case <-*startSigChan:
			}
		}
	}

	i.logger.Info("starting worker", "worker", idx)

	clientPtr := i.client.Load()
	if clientPtr == nil || *clientPtr == nil {
		i.logger.Error("worker: client not initialized", "worker", idx)
		return
	}
	client := *clientPtr

	resetChan := i.reset.Load()

	// Resolved here, not in the drainer: WriteAPI() mutates client state.
	writeAPI := client.WriteAPI(cfg.Org, cfg.Bucket)
	errCh := writeAPI.Errors()

	// Not tied to ctx: the drainer must outlive cancellation until the worker is
	// clear of WritePoint, or a pending error strands the worker there.
	stopErrDrain := make(chan struct{})
	drainDone := make(chan struct{})
	go i.drainWriteErrors(stopErrDrain, errCh, idx, drainDone)
	var stopOnce sync.Once
	stopDrain := func() {
		stopOnce.Do(func() { close(stopErrDrain) })
		<-drainDone
	}
	defer stopDrain()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				i.logger.Warn("worker err", "worker", idx, "err", ctx.Err())
			}
			i.logger.Info("worker terminating", "worker", idx)
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

			if dropped := dropNonFinite(ev); len(dropped) > 0 {
				if cfg.EnableMetrics {
					influxNonFiniteDropped.
						WithLabelValues(cfg.Name, ev.Name).
						Add(float64(len(dropped)))
				}
				i.logNonFinite(idx, ev.Name, dropped)
			}

			if len(ev.Values) == 0 && len(ev.Deletes) == 0 {
				continue
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
				writeAPI.
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
				writeAPI.
					WritePoint(influxdb2.NewPoint(ev.Name, tags, values, time.Unix(0, ev.Timestamp)))
			}
		case <-*resetChan:
			firstStart = false
			i.logger.Info("resetting worker", "worker", idx)
			stopDrain() // else goto START leaks a drainer per reset
			goto START
		}
	}
}

// drainWriteErrors consumes the client's async error channel for one worker
// generation. It must not run on the goroutine that calls WritePoint: the
// client's encoding-error path blocks sending on this single-slot channel.
//
// stop is closed by the worker only once it is no longer inside WritePoint.
// Waiting on that rather than on ctx matters: if the worker is blocked
// delivering an error and this goroutine returned on ctx cancellation, the
// error would go undrained and the worker would never leave WritePoint, so
// Close()'s WaitGroup wait would hang.
func (i *influxDBOutput) drainWriteErrors(stop <-chan struct{}, errCh <-chan error, idx int, done chan struct{}) {
	defer close(done)
	for {
		// Prefer a pending error over shutdown: a queued error means a producer
		// is blocked on it.
		select {
		case err, ok := <-errCh:
			if !ok {
				return
			}
			i.logger.Error("worker write error", "worker", idx, "err", err)
			continue
		default:
		}
		select {
		case <-stop:
			return
		case err, ok := <-errCh:
			if !ok {
				return
			}
			i.logger.Error("worker write error", "worker", idx, "err", err)
		}
	}
}

// nonFiniteLogState rate-limits the "dropped non-finite values" log: a dark lane
// reports -Inf on every sample, so per-event logging runs to thousands of lines
// a day. Exact counts live in the metric.
type nonFiniteLogState struct {
	mu         sync.Mutex
	first      bool
	nextLog    time.Time
	suppressed int
	// keyed "measurement/field": the summary spans measurements, so an
	// unqualified field name would be attributed to the wrong one.
	fields map[string]struct{}
}

const nonFiniteLogInterval = 5 * time.Minute

func (i *influxDBOutput) logNonFinite(idx int, measurement string, dropped []string) {
	st := &i.nonFiniteLog
	st.mu.Lock()
	if st.fields == nil {
		st.fields = make(map[string]struct{})
	}
	for _, f := range dropped {
		st.fields[measurement+"/"+f] = struct{}{}
	}
	st.suppressed += len(dropped)

	now := time.Now()
	if st.first && now.Before(st.nextLog) {
		st.mu.Unlock()
		return
	}
	firstTime := !st.first
	st.first = true
	st.nextLog = now.Add(nonFiniteLogInterval)
	count := st.suppressed
	st.suppressed = 0
	fields := slices.Sorted(maps.Keys(st.fields))
	clear(st.fields)
	st.mu.Unlock()

	if firstTime {
		i.logger.Warn("dropping non-finite values (NaN/Inf) that cannot be encoded in line protocol; "+
			"further occurrences are summarized every "+nonFiniteLogInterval.String()+
			" -- see gnmic_influxdb_output_non_finite_values_dropped_total for exact counts",
			"worker", idx,
			"measurement", measurement,
			"fields", strings.Join(dropped, ","),
		)
		return
	}
	i.logger.Warn("dropped non-finite values",
		"worker", idx,
		"fields", strings.Join(fields, ","),
		"count", count,
		"interval", nonFiniteLogInterval.String(),
	)
}

// dropNonFinite removes NaN/±Inf float fields, which line protocol cannot
// encode, and returns their names. Other fields on the point are kept.
func dropNonFinite(ev *formatters.EventMsg) []string {
	var dropped []string
	for k, v := range ev.Values {
		switch f := v.(type) {
		case float64:
			if math.IsNaN(f) || math.IsInf(f, 0) {
				delete(ev.Values, k)
				dropped = append(dropped, k)
			}
		case float32:
			if f64 := float64(f); math.IsNaN(f64) || math.IsInf(f64, 0) {
				delete(ev.Values, k)
				dropped = append(dropped, k)
			}
		}
	}
	return dropped
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
	// Org/Bucket select the client's WriteAPI, and each WriteAPI has its own
	// error channel. Workers bind one for their lifetime, so a change here must
	// restart them or the new API's errors go undrained.
	return old.URL != new.URL ||
		old.Org != new.Org ||
		old.Bucket != new.Bucket ||
		old.Token != new.Token ||
		old.BatchSize != new.BatchSize ||
		old.FlushTimer != new.FlushTimer ||
		old.UseGzip != new.UseGzip ||
		old.EnableTLS != new.EnableTLS ||
		!old.TLS.Equal(new.TLS) ||
		old.TimestampPrecision != new.TimestampPrecision ||
		old.Debug != new.Debug
}
