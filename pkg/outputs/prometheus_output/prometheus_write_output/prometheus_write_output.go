// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/prompb"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	promcom "github.com/openconfig/gnmic/pkg/outputs/prometheus_output"
	gutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
)

const (
	outputType                        = "prometheus_write"
	loggingPrefix                     = "[prometheus_write_output:%s] "
	defaultTimeout                    = 10 * time.Second
	defaultWriteInterval              = 10 * time.Second
	defaultMetadataWriteInterval      = time.Minute
	defaultBufferSize                 = 1000
	defaultMaxTSPerWrite              = 500
	defaultMaxMetaDataEntriesPerWrite = 500
	defaultMetricHelp                 = "gNMIc generated metric"
	userAgent                         = "gNMIc prometheus write"
	defaultNumWorkers                 = 1
	defaultNumWriters                 = 1
)

func init() {
	outputs.Register(outputType,
		func() outputs.Output {
			return &promWriteOutput{}
		})
}

type promWriteOutput struct {
	outputs.BaseOutput

	cfg          *atomic.Pointer[config]
	dynCfg       *atomic.Pointer[dynConfig]
	httpClient   *atomic.Pointer[http.Client]
	timeSeriesCh *atomic.Pointer[chan *prompb.TimeSeries]

	logger      *log.Logger
	eventChan   chan *formatters.EventMsg
	msgChan     chan *outputs.ProtoMsg
	buffDrainCh chan struct{}

	m             *sync.Mutex
	metadataCache map[string]prompb.MetricMetadata

	rootCtx  context.Context
	cancelFn context.CancelFunc
	wg       *sync.WaitGroup

	reg   *prometheus.Registry
	store store.Store[any]
}

type config struct {
	Name                  string            `mapstructure:"name,omitempty" json:"name,omitempty"`
	URL                   string            `mapstructure:"url,omitempty" json:"url,omitempty"`
	Timeout               time.Duration     `mapstructure:"timeout,omitempty" json:"timeout,omitempty"`
	Headers               map[string]string `mapstructure:"headers,omitempty" json:"headers,omitempty"`
	Authentication        *auth             `mapstructure:"authentication,omitempty" json:"authentication,omitempty"`
	Authorization         *authorization    `mapstructure:"authorization,omitempty" json:"authorization,omitempty"`
	TLS                   *types.TLSConfig  `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	Interval              time.Duration     `mapstructure:"interval,omitempty" json:"interval,omitempty"`
	BufferSize            int               `mapstructure:"buffer-size,omitempty" json:"buffer-size,omitempty"`
	MaxTimeSeriesPerWrite int               `mapstructure:"max-time-series-per-write,omitempty" json:"max-time-series-per-write,omitempty"`
	MaxRetries            int               `mapstructure:"max-retries,omitempty" json:"max-retries,omitempty"`
	Metadata              *metadata         `mapstructure:"metadata,omitempty" json:"metadata,omitempty"`
	Debug                 bool              `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	//
	MetricPrefix           string   `mapstructure:"metric-prefix,omitempty" json:"metric-prefix,omitempty"`
	AppendSubscriptionName bool     `mapstructure:"append-subscription-name,omitempty" json:"append-subscription-name,omitempty"`
	AddTarget              string   `mapstructure:"add-target,omitempty" json:"add-target,omitempty"`
	TargetTemplate         string   `mapstructure:"target-template,omitempty" json:"target-template,omitempty"`
	StringsAsLabels        bool     `mapstructure:"strings-as-labels,omitempty" json:"strings-as-labels,omitempty"`
	EventProcessors        []string `mapstructure:"event-processors,omitempty" json:"event-processors,omitempty"`
	NumWorkers             int      `mapstructure:"num-workers,omitempty" json:"num-workers,omitempty"`
	NumWriters             int      `mapstructure:"num-writers,omitempty" json:"num-writers,omitempty"`
	EnableMetrics          bool     `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
}

type dynConfig struct {
	targetTpl *template.Template
	evps      []formatters.EventProcessor
	mb        *promcom.MetricBuilder
}

type auth struct {
	Username string `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password string `mapstructure:"password,omitempty" json:"password,omitempty"`
}

type authorization struct {
	Type        string `mapstructure:"type,omitempty" json:"type,omitempty"`
	Credentials string `mapstructure:"credentials,omitempty" json:"credentials,omitempty"`
}

type metadata struct {
	Include            bool          `mapstructure:"include,omitempty" json:"include,omitempty"`
	Interval           time.Duration `mapstructure:"interval,omitempty" json:"interval,omitempty"`
	MaxEntriesPerWrite int           `mapstructure:"max-entries-per-write,omitempty" json:"max-entries-per-write,omitempty"`
}

func (p *promWriteOutput) init() {
	p.cfg = new(atomic.Pointer[config])
	p.dynCfg = new(atomic.Pointer[dynConfig])
	p.httpClient = new(atomic.Pointer[http.Client])
	p.timeSeriesCh = new(atomic.Pointer[chan *prompb.TimeSeries])
	p.wg = new(sync.WaitGroup)
	p.m = new(sync.Mutex)
	p.logger = log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags)
	p.eventChan = make(chan *formatters.EventMsg)
	p.msgChan = make(chan *outputs.ProtoMsg)
	p.buffDrainCh = make(chan struct{}, 1)
	p.metadataCache = make(map[string]prompb.MetricMetadata)
}

func (p *promWriteOutput) buildEventProcessors(cfg *config) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(p.store)
	if err != nil {
		return nil, err
	}
	return formatters.MakeEventProcessors(p.logger, cfg.EventProcessors, ps, tcs, acts)
}

func (p *promWriteOutput) setLogger(logger *log.Logger) {
	if logger != nil && p.logger != nil {
		p.logger.SetOutput(logger.Writer())
		p.logger.SetFlags(logger.Flags())
	}
}

func (p *promWriteOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	p.init()

	ncfg := new(config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}
	if ncfg.URL == "" {
		return errors.New("missing url field")
	}
	_, err = url.Parse(ncfg.URL)
	if err != nil {
		return err
	}
	if ncfg.Name == "" {
		ncfg.Name = name
	}
	p.logger.SetPrefix(fmt.Sprintf(loggingPrefix, ncfg.Name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	p.store = options.Store

	// apply logger
	p.setLogger(options.Logger)

	// set defaults
	p.setDefaultsFor(ncfg)

	p.cfg.Store(ncfg)

	// initialize registry
	p.reg = options.Registry
	err = p.registerMetrics()
	if err != nil {
		return err
	}

	// prep dynamic config
	dc := new(dynConfig)

	// initialize event processors
	dc.evps, err = p.buildEventProcessors(ncfg)
	if err != nil {
		return err
	}

	// initialize target template
	if ncfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if ncfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", ncfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	dc.mb = &promcom.MetricBuilder{
		Prefix:                 ncfg.MetricPrefix,
		AppendSubscriptionName: ncfg.AppendSubscriptionName,
		StringsAsLabels:        ncfg.StringsAsLabels,
	}

	p.dynCfg.Store(dc)

	// initialize buffer chan
	timeSeriesCh := make(chan *prompb.TimeSeries, ncfg.BufferSize)
	p.timeSeriesCh.Store(&timeSeriesCh)

	cl, err := p.createHTTPClientFor(ncfg)
	if err != nil {
		return err
	}
	p.httpClient.Store(cl)

	p.rootCtx = ctx
	var wctx context.Context
	wctx, p.cancelFn = context.WithCancel(p.rootCtx)

	p.wg.Add(ncfg.NumWorkers)
	for i := 0; i < ncfg.NumWorkers; i++ {
		go p.worker(wctx)
	}

	p.wg.Add(ncfg.NumWriters)
	for i := 0; i < ncfg.NumWriters; i++ {
		go p.writer(wctx)
	}

	p.wg.Add(1)
	go p.metadataWriter(wctx)

	p.logger.Printf("initialized prometheus write output %s: %s", ncfg.Name, p.String())
	return nil
}

func (p *promWriteOutput) Update(ctx context.Context, cfg map[string]any) error {
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	if newCfg.URL == "" {
		return errors.New("missing url field")
	}
	if _, err := url.Parse(newCfg.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	p.setDefaultsFor(newCfg)

	currCfg := p.cfg.Load()

	swapChannel := channelNeedsSwap(currCfg, newCfg)
	restartWorkers := needsWorkerRestart(currCfg, newCfg)
	rebuildClient := needsClientRebuild(currCfg, newCfg)
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0

	// Rebuild dynamic config
	dc := new(dynConfig)

	// target template
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

	// metric builder
	dc.mb = &promcom.MetricBuilder{
		Prefix:                 newCfg.MetricPrefix,
		AppendSubscriptionName: newCfg.AppendSubscriptionName,
		StringsAsLabels:        newCfg.StringsAsLabels,
	}

	// rebuild processors ?
	prevDC := p.dynCfg.Load()
	if rebuildProcessors {
		dc.evps, err = p.buildEventProcessors(newCfg)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}

	// store new dynamic config
	p.dynCfg.Store(dc)

	// rebuild HTTP client if needed
	if rebuildClient {
		newClient, err := p.createHTTPClientFor(newCfg)
		if err != nil {
			return err
		}
		oldClient := p.httpClient.Swap(newClient)
		if oldClient != nil {
			oldClient.CloseIdleConnections()
		}
	}

	// store new config
	p.cfg.Store(newCfg)

	if swapChannel || restartWorkers {
		var newChan chan *prompb.TimeSeries
		if swapChannel {
			newChan = make(chan *prompb.TimeSeries, newCfg.BufferSize)
		} else {
			newChan = *p.timeSeriesCh.Load()
		}

		runCtx, cancel := context.WithCancel(p.rootCtx)
		newWG := new(sync.WaitGroup)

		// save old pointers
		oldCancel := p.cancelFn
		oldWG := p.wg
		oldTSCh := *p.timeSeriesCh.Load()

		// swap
		p.cancelFn = cancel
		p.wg = newWG
		p.timeSeriesCh.Store(&newChan)

		// restart workers
		p.wg.Add(newCfg.NumWorkers)
		for i := 0; i < newCfg.NumWorkers; i++ {
			go p.worker(runCtx)
		}

		p.wg.Add(newCfg.NumWriters)
		for i := 0; i < newCfg.NumWriters; i++ {
			go p.writer(runCtx)
		}

		p.wg.Add(1)
		go p.metadataWriter(runCtx)

		// cancel old workers
		if oldCancel != nil {
			oldCancel()
		}
		if oldWG != nil {
			oldWG.Wait()
		}

		if swapChannel {
			// best effort drain old channel
		OUTER_LOOP:
			for {
				select {
				case ts, ok := <-oldTSCh:
					if !ok {
						break
					}
					select {
					case newChan <- ts:
					default:
						// new channel full, drop message
					}
				default:
					break OUTER_LOOP
				}
			}
		}
	}

	p.logger.Printf("updated prometheus write output: %s", p.String())
	return nil
}

func (p *promWriteOutput) Validate(cfg map[string]any) error {
	ncfg := new(config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}
	if ncfg.URL == "" {
		return errors.New("missing url field")
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

func (p *promWriteOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	cfg := p.cfg.Load()

	wctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return
	case p.msgChan <- outputs.NewProtoMsg(rsp, meta):
	case <-wctx.Done():
		if cfg.Debug {
			p.logger.Printf("writing expired after %s", cfg.Timeout)
		}
		return
	}
}

func (p *promWriteOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	dc := p.dynCfg.Load()
	if dc == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
		var evs = []*formatters.EventMsg{ev}
		for _, proc := range dc.evps {
			evs = proc.Apply(evs...)
		}
		for _, pev := range evs {
			p.eventChan <- pev
		}
	}
}

func (p *promWriteOutput) Close() error {
	if p.cancelFn != nil {
		p.cancelFn()
	}
	p.wg.Wait()

	client := p.httpClient.Load()
	if client != nil {
		client.CloseIdleConnections()
	}
	p.logger.Printf("closed prometheus write output: %s", p.String())
	return nil
}

func (p *promWriteOutput) String() string {
	cfg := p.cfg.Load()
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (p *promWriteOutput) worker(ctx context.Context) {
	defer p.wg.Done()
	defer p.logger.Printf("worker stopped")
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-p.eventChan:
			p.workerHandleEvent(ev)
		case m := <-p.msgChan:
			p.workerHandleProto(ctx, m)
		}
	}
}

func (p *promWriteOutput) workerHandleProto(_ context.Context, m *outputs.ProtoMsg) {
	cfg := p.cfg.Load()
	dc := p.dynCfg.Load()

	pmsg := m.GetMsg()
	switch pmsg := pmsg.(type) {
	case *gnmi.SubscribeResponse:
		meta := m.GetMeta()
		measName := "default"
		if subName, ok := meta["subscription-name"]; ok {
			measName = subName
		}
		var err error
		pmsg, err = outputs.AddSubscriptionTarget(pmsg, m.GetMeta(), cfg.AddTarget, dc.targetTpl)
		if err != nil {
			p.logger.Printf("failed to add target to the response: %v", err)
		}
		events, err := formatters.ResponseToEventMsgs(measName, pmsg, meta, dc.evps...)
		if err != nil {
			p.logger.Printf("failed to convert message to event: %v", err)
			return
		}
		for _, ev := range events {
			p.workerHandleEvent(ev)
		}
	}
}

func (p *promWriteOutput) workerHandleEvent(ev *formatters.EventMsg) {
	cfg := p.cfg.Load()
	dc := p.dynCfg.Load()
	tsCh := p.timeSeriesCh.Load()

	if cfg.Debug {
		p.logger.Printf("got event to buffer: %+v", ev)
	}
	for _, pts := range dc.mb.TimeSeriesFromEvent(ev) {
		if len(*tsCh) >= cfg.BufferSize {
			if cfg.Debug {
				p.logger.Printf("buffer size reached, triggering write")
			}
			p.buffDrainCh <- struct{}{}
		}
		// populate metadata cache
		p.m.Lock()
		if cfg.Debug {
			p.logger.Printf("saving metrics metadata")
		}
		p.metadataCache[pts.Name] = prompb.MetricMetadata{
			Type:             prompb.MetricMetadata_COUNTER,
			MetricFamilyName: pts.Name,
			Help:             defaultMetricHelp,
		}
		p.m.Unlock()
		// write time series to buffer
		if cfg.Debug {
			p.logger.Printf("writing TimeSeries to buffer")
		}
		*tsCh <- pts.TS
	}
}

func (p *promWriteOutput) setDefaultsFor(c *config) {
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.Interval <= 0 {
		c.Interval = defaultWriteInterval
	}
	if c.BufferSize <= 0 {
		c.BufferSize = defaultBufferSize
	}
	if c.NumWorkers <= 0 {
		c.NumWorkers = defaultNumWorkers
	}
	if c.NumWriters <= 0 {
		c.NumWriters = defaultNumWriters
	}
	if c.MaxTimeSeriesPerWrite <= 0 {
		c.MaxTimeSeriesPerWrite = defaultMaxTSPerWrite
	}
	if c.Metadata == nil {
		c.Metadata = &metadata{
			Include:            true,
			Interval:           defaultMetadataWriteInterval,
			MaxEntriesPerWrite: defaultMaxMetaDataEntriesPerWrite,
		}
		return
	}
	if c.Metadata.Include {
		if c.Metadata.Interval <= 0 {
			c.Metadata.Interval = defaultMetadataWriteInterval
		}
		if c.Metadata.MaxEntriesPerWrite <= 0 {
			c.Metadata.MaxEntriesPerWrite = defaultMaxMetaDataEntriesPerWrite
		}
	}
}

// Helper functions

func channelNeedsSwap(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.BufferSize != nw.BufferSize
}

func needsWorkerRestart(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.NumWorkers != nw.NumWorkers ||
		old.NumWriters != nw.NumWriters ||
		old.Interval != nw.Interval ||
		metadataChanged(old.Metadata, nw.Metadata)
}

func metadataChanged(old, nw *metadata) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.Include != nw.Include ||
		old.Interval != nw.Interval ||
		old.MaxEntriesPerWrite != nw.MaxEntriesPerWrite
}

func needsClientRebuild(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.URL != nw.URL ||
		old.Timeout != nw.Timeout ||
		!old.TLS.Equal(nw.TLS) ||
		!authEq(old.Authentication, nw.Authentication) ||
		!authzEq(old.Authorization, nw.Authorization)
}

func authEq(a, b *auth) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Username == b.Username && a.Password == b.Password
}

func authzEq(a, b *authorization) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Type == b.Type && a.Credentials == b.Credentials
}
