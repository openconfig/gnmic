// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
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
	"sync"
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
			return &promWriteOutput{
				cfg:           &config{},
				logger:        log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
				eventChan:     make(chan *formatters.EventMsg),
				msgChan:       make(chan *outputs.ProtoMsg),
				buffDrainCh:   make(chan struct{}),
				m:             new(sync.Mutex),
				metadataCache: make(map[string]prompb.MetricMetadata),
			}
		})
}

type promWriteOutput struct {
	cfg    *config
	logger *log.Logger

	httpClient   *http.Client
	eventChan    chan *formatters.EventMsg
	msgChan      chan *outputs.ProtoMsg
	timeSeriesCh chan *prompb.TimeSeries
	buffDrainCh  chan struct{}
	mb           *promcom.MetricBuilder

	m             *sync.Mutex
	metadataCache map[string]prompb.MetricMetadata

	evps      []formatters.EventProcessor
	targetTpl *template.Template
	cfn       context.CancelFunc

	reg *prometheus.Registry
	// TODO:
	// gnmiCache *cache.GnmiOutputCache
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

func (p *promWriteOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, p.cfg)
	if err != nil {
		return err
	}
	if p.cfg.URL == "" {
		return errors.New("missing url field")
	}
	_, err = url.Parse(p.cfg.URL)
	if err != nil {
		return err
	}
	if p.cfg.Name == "" {
		p.cfg.Name = name
	}
	p.logger.SetPrefix(fmt.Sprintf(loggingPrefix, p.cfg.Name))

	for _, opt := range opts {
		if err := opt(p); err != nil {
			return err
		}
	}

	err = p.registerMetrics()
	if err != nil {
		return err
	}

	if p.cfg.TargetTemplate == "" {
		p.targetTpl = outputs.DefaultTargetTemplate
	} else if p.cfg.AddTarget != "" {
		p.targetTpl, err = gtemplate.CreateTemplate("target-template", p.cfg.TargetTemplate)
		if err != nil {
			return err
		}
		p.targetTpl = p.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	err = p.setDefaults()
	if err != nil {
		return err
	}

	p.mb = &promcom.MetricBuilder{
		Prefix:                 p.cfg.MetricPrefix,
		AppendSubscriptionName: p.cfg.AppendSubscriptionName,
		StringsAsLabels:        p.cfg.StringsAsLabels,
	}

	// initialize buffer chan
	p.timeSeriesCh = make(chan *prompb.TimeSeries, p.cfg.BufferSize)
	err = p.createHTTPClient()
	if err != nil {
		return err
	}

	ctx, p.cfn = context.WithCancel(ctx)
	for i := 0; i < p.cfg.NumWorkers; i++ {
		go p.worker(ctx)
	}
	for i := 0; i < p.cfg.NumWriters; i++ {
		go p.writer(ctx)
	}
	go p.metadataWriter(ctx)
	p.logger.Printf("initialized prometheus write output %s: %s", p.cfg.Name, p.String())
	return nil
}

func (p *promWriteOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	wctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return
	case p.msgChan <- outputs.NewProtoMsg(rsp, meta):
	case <-wctx.Done():
		if p.cfg.Debug {
			p.logger.Printf("writing expired after %s", p.cfg.Timeout)
		}
		return
	}
}

func (p *promWriteOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	select {
	case <-ctx.Done():
		return
	default:
		var evs = []*formatters.EventMsg{ev}
		for _, proc := range p.evps {
			evs = proc.Apply(evs...)
		}
		for _, pev := range evs {
			p.eventChan <- pev
		}
	}
}

func (p *promWriteOutput) Close() error {
	if p.cfn == nil {
		return nil
	}
	p.cfn()
	return nil
}

func (p *promWriteOutput) RegisterMetrics(reg *prometheus.Registry) {
	if !p.cfg.EnableMetrics {
		return
	}
	p.reg = reg
}

func (p *promWriteOutput) String() string {
	b, err := json.Marshal(p.cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (p *promWriteOutput) SetLogger(logger *log.Logger) {
	if logger != nil && p.logger != nil {
		p.logger.SetOutput(logger.Writer())
		p.logger.SetFlags(logger.Flags())
	}
}

func (p *promWriteOutput) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) error {
	var err error
	p.evps, err = formatters.MakeEventProcessors(
		logger,
		p.cfg.EventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return err
	}
	return nil
}

func (p *promWriteOutput) SetName(name string) {
	if p.cfg.Name == "" {
		p.cfg.Name = name
	}
}

func (p *promWriteOutput) SetClusterName(_ string) {}

func (p *promWriteOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}

//

func (p *promWriteOutput) worker(ctx context.Context) {
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
	pmsg := m.GetMsg()
	switch pmsg := pmsg.(type) {
	case *gnmi.SubscribeResponse:
		meta := m.GetMeta()
		measName := "default"
		if subName, ok := meta["subscription-name"]; ok {
			measName = subName
		}
		var err error
		pmsg, err = outputs.AddSubscriptionTarget(pmsg, m.GetMeta(), p.cfg.AddTarget, p.targetTpl)
		if err != nil {
			p.logger.Printf("failed to add target to the response: %v", err)
		}
		events, err := formatters.ResponseToEventMsgs(measName, pmsg, meta, p.evps...)
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
	if p.cfg.Debug {
		p.logger.Printf("got event to buffer: %+v", ev)
	}
	for _, pts := range p.mb.TimeSeriesFromEvent(ev) {
		if len(p.timeSeriesCh) >= p.cfg.BufferSize {
			if p.cfg.Debug {
				p.logger.Printf("buffer size reached, triggering write")
			}
			p.buffDrainCh <- struct{}{}
		}
		// populate metadata cache
		p.m.Lock()
		if p.cfg.Debug {
			p.logger.Printf("saving metrics metadata")
		}
		p.metadataCache[pts.Name] = prompb.MetricMetadata{
			Type:             prompb.MetricMetadata_COUNTER,
			MetricFamilyName: pts.Name,
			Help:             defaultMetricHelp,
		}
		p.m.Unlock()
		// write time series to buffer
		if p.cfg.Debug {
			p.logger.Printf("writing TimeSeries to buffer")
		}
		p.timeSeriesCh <- pts.TS
	}
}

func (p *promWriteOutput) setDefaults() error {
	if p.cfg.Timeout <= 0 {
		p.cfg.Timeout = defaultTimeout
	}
	if p.cfg.Interval <= 0 {
		p.cfg.Interval = defaultWriteInterval
	}
	if p.cfg.BufferSize <= 0 {
		p.cfg.BufferSize = defaultBufferSize
	}
	if p.cfg.NumWorkers <= 0 {
		p.cfg.NumWorkers = defaultNumWorkers
	}
	if p.cfg.NumWriters <= 0 {
		p.cfg.NumWriters = defaultNumWriters
	}
	if p.cfg.MaxTimeSeriesPerWrite <= 0 {
		p.cfg.MaxTimeSeriesPerWrite = defaultMaxTSPerWrite
	}
	if p.cfg.Metadata == nil {
		p.cfg.Metadata = &metadata{
			Include:            true,
			Interval:           defaultMetadataWriteInterval,
			MaxEntriesPerWrite: defaultMaxMetaDataEntriesPerWrite,
		}
		return nil
	}
	if p.cfg.Metadata.Include {
		if p.cfg.Metadata.Interval <= 0 {
			p.cfg.Metadata.Interval = defaultMetadataWriteInterval
		}
		if p.cfg.Metadata.MaxEntriesPerWrite <= 0 {
			p.cfg.Metadata.MaxEntriesPerWrite = defaultMaxMetaDataEntriesPerWrite
		}
	}
	return nil
}
