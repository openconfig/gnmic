package prometheus_write_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/prompb"

	promcom "github.com/karimra/gnmic/outputs/prometheus_output"
	"google.golang.org/protobuf/proto"
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
)

func init() {
	outputs.Register(outputType,
		func() outputs.Output {
			return &promWriteOutput{
				Cfg:           &config{},
				logger:        log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
				eventChan:     make(chan *formatters.EventMsg),
				buffDrainCh:   make(chan struct{}),
				m:             new(sync.Mutex),
				metadataCache: make(map[string]prompb.MetricMetadata),
			}
		})
}

type promWriteOutput struct {
	Cfg    *config
	logger *log.Logger

	httpClient   *http.Client
	eventChan    chan *formatters.EventMsg
	timeSeriesCh chan *prompb.TimeSeries
	buffDrainCh  chan struct{}
	mb           *promcom.MetricBuilder

	m             *sync.Mutex
	metadataCache map[string]prompb.MetricMetadata

	evps      []formatters.EventProcessor
	targetTpl *template.Template
	cfn       context.CancelFunc
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
	TLS                   *tls              `mapstructure:"tls,omitempty" json:"tls,omitempty"`
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
}

type auth struct {
	Username string `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password string `mapstructure:"password,omitempty" json:"password,omitempty"`
}

type authorization struct {
	Type        string `mapstructure:"type,omitempty" json:"type,omitempty"`
	Credentials string `mapstructure:"credentials,omitempty" json:"credentials,omitempty"`
}

type tls struct {
	CAFile     string `mapstructure:"ca-file,omitempty" json:"ca-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty" json:"cert-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty" json:"key-file,omitempty"`
	SkipVerify bool   `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty"`
}

type metadata struct {
	Include            bool          `mapstructure:"include,omitempty" json:"include,omitempty"`
	Interval           time.Duration `mapstructure:"interval,omitempty" json:"interval,omitempty"`
	MaxEntriesPerWrite int           `mapstructure:"max-entries-per-write,omitempty" json:"max-entries-per-write,omitempty"`
}

func (p *promWriteOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, p.Cfg)
	if err != nil {
		return err
	}
	if p.Cfg.URL == "" {
		return errors.New("missing url field")
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.Cfg.Name == "" {
		p.Cfg.Name = name
	}
	p.logger.SetPrefix(fmt.Sprintf(loggingPrefix, p.Cfg.Name))
	if p.Cfg.TargetTemplate == "" {
		p.targetTpl = outputs.DefaultTargetTemplate
	} else if p.Cfg.AddTarget != "" {
		p.targetTpl, err = utils.CreateTemplate("target-template", p.Cfg.TargetTemplate)
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
		Prefix:                 p.Cfg.MetricPrefix,
		AppendSubscriptionName: p.Cfg.AppendSubscriptionName,
		StringsAsLabels:        p.Cfg.StringsAsLabels,
	}

	// initialize buffer chan
	p.timeSeriesCh = make(chan *prompb.TimeSeries, p.Cfg.BufferSize)
	err = p.createHTTPClient()
	if err != nil {
		return err
	}

	ctx, p.cfn = context.WithCancel(ctx)
	go p.worker(ctx)
	go p.writer(ctx)
	go p.metadataWriter(ctx)
	p.logger.Printf("initialized prometheus write output %s: %s", p.Cfg.Name, p.String())
	return nil
}

func (p *promWriteOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	switch rsp := rsp.(type) {
	case *gnmi.SubscribeResponse:
		measName := "default"
		if subName, ok := meta["subscription-name"]; ok {
			measName = subName
		}
		err := outputs.AddSubscriptionTarget(rsp, meta, p.Cfg.AddTarget, p.targetTpl)
		if err != nil {
			p.logger.Printf("failed to add target to the response: %v", err)
		}

		events, err := formatters.ResponseToEventMsgs(measName, rsp, meta, p.evps...)
		if err != nil {
			p.logger.Printf("failed to convert message to event: %v", err)
			return
		}
		for _, ev := range events {
			select {
			case <-ctx.Done():
				return
			case p.eventChan <- ev:
			}
		}
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

func (p *promWriteOutput) RegisterMetrics(_ *prometheus.Registry) {}

func (p *promWriteOutput) String() string {
	b, err := json.Marshal(p)
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
	acts map[string]map[string]interface{}) {
	for _, epName := range p.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType],
					formatters.WithLogger(logger),
					formatters.WithTargets(tcs),
					formatters.WithActions(acts))
				if err != nil {
					p.logger.Printf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
					continue
				}
				p.evps = append(p.evps, ep)
				p.logger.Printf("added event processor '%s' of type=%s to prometheus output", epName, epType)
				continue
			}
			p.logger.Printf("%q event processor has an unknown type=%q", epName, epType)
			continue
		}
		p.logger.Printf("%q event processor not found!", epName)
	}
}

func (p *promWriteOutput) SetName(name string) {
	if p.Cfg.Name == "" {
		p.Cfg.Name = name
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
			if p.Cfg.Debug {
				p.logger.Printf("got event to buffer: %+v", ev)
			}
			for _, pts := range p.mb.TimeSeriesFromEvent(ev) {
				if len(p.timeSeriesCh) >= p.Cfg.BufferSize {
					//if p.Cfg.Debug {
					p.logger.Printf("buffer size reached, triggering write")
					// }
					p.buffDrainCh <- struct{}{}
				}
				// populate metadata cache
				p.m.Lock()
				if p.Cfg.Debug {
					p.logger.Printf("saving metrics metadata")
				}
				p.metadataCache[pts.Name] = prompb.MetricMetadata{
					Type:             prompb.MetricMetadata_COUNTER,
					MetricFamilyName: pts.Name,
					Help:             defaultMetricHelp,
				}
				p.m.Unlock()
				// write time series to buffer
				if p.Cfg.Debug {
					p.logger.Printf("writing TimeSeries to buffer")
				}
				p.timeSeriesCh <- pts.TS
			}
		}
	}
}

func (p *promWriteOutput) setDefaults() error {
	if p.Cfg.Timeout <= 0 {
		p.Cfg.Timeout = defaultTimeout
	}
	if p.Cfg.Interval <= 0 {
		p.Cfg.Interval = defaultWriteInterval
	}
	if p.Cfg.BufferSize <= 0 {
		p.Cfg.BufferSize = defaultBufferSize
	}
	if p.Cfg.MaxTimeSeriesPerWrite <= 0 {
		p.Cfg.MaxTimeSeriesPerWrite = defaultMaxTSPerWrite
	}
	if p.Cfg.Metadata == nil {
		p.Cfg.Metadata = &metadata{
			Include:            true,
			Interval:           defaultMetadataWriteInterval,
			MaxEntriesPerWrite: defaultMaxMetaDataEntriesPerWrite,
		}
		return nil
	}
	if p.Cfg.Metadata.Include {
		if p.Cfg.Metadata.Interval <= 0 {
			p.Cfg.Metadata.Interval = defaultMetadataWriteInterval
		}
		if p.Cfg.Metadata.MaxEntriesPerWrite <= 0 {
			p.Cfg.Metadata.MaxEntriesPerWrite = defaultMaxMetaDataEntriesPerWrite
		}
	}
	return nil
}
