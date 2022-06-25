package prometheus_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	"github.com/karimra/gnmic/cache"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	promcom "github.com/karimra/gnmic/outputs/prometheus_output"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
	"google.golang.org/protobuf/proto"
)

const (
	outputType        = "prometheus"
	defaultListen     = ":9804"
	defaultPath       = "/metrics"
	defaultExpiration = time.Minute
	defaultMetricHelp = "gNMIc generated metric"
	loggingPrefix     = "[prometheus_output:%s] "
	// this is used to timeout the collection method
	// in case it drags for too long
	defaultTimeout = 10 * time.Second
)

type promMetric struct {
	name   string
	labels []prompb.Label
	time   *time.Time
	value  float64
	// addedAt is used to expire metrics if the time field is not initialized
	// this happens when ExportTimestamp == false
	addedAt time.Time
}

func init() {
	outputs.Register(outputType, func() outputs.Output {
		return &prometheusOutput{
			Cfg:       &config{},
			eventChan: make(chan *formatters.EventMsg),
			wg:        new(sync.WaitGroup),
			entries:   make(map[uint64]*promMetric),
			logger:    log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type prometheusOutput struct {
	Cfg       *config
	logger    *log.Logger
	eventChan chan *formatters.EventMsg

	wg     *sync.WaitGroup
	server *http.Server
	sync.Mutex
	entries map[uint64]*promMetric

	mb           *promcom.MetricBuilder
	evps         []formatters.EventProcessor
	consulClient *api.Client

	targetTpl *template.Template

	gnmiCache *cache.GnmiCache
}

type config struct {
	Name                   string               `mapstructure:"name,omitempty"`
	Listen                 string               `mapstructure:"listen,omitempty"`
	Path                   string               `mapstructure:"path,omitempty"`
	Expiration             time.Duration        `mapstructure:"expiration,omitempty"`
	MetricPrefix           string               `mapstructure:"metric-prefix,omitempty"`
	AppendSubscriptionName bool                 `mapstructure:"append-subscription-name,omitempty"`
	ExportTimestamps       bool                 `mapstructure:"export-timestamps,omitempty"`
	OverrideTimestamps     bool                 `mapstructure:"override-timestamps,omitempty"`
	AddTarget              string               `mapstructure:"add-target,omitempty"`
	TargetTemplate         string               `mapstructure:"target-template,omitempty"`
	StringsAsLabels        bool                 `mapstructure:"strings-as-labels,omitempty"`
	Debug                  bool                 `mapstructure:"debug,omitempty"`
	EventProcessors        []string             `mapstructure:"event-processors,omitempty"`
	ServiceRegistration    *serviceRegistration `mapstructure:"service-registration,omitempty"`
	GnmiCache              bool                 `mapstructure:"gnmi-cache,omitempty"`
	Timeout                time.Duration        `mapstructure:"timeout,omitempty"`

	clusterName string
	address     string
	port        int
}

func (p *prometheusOutput) String() string {
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func (p *prometheusOutput) SetLogger(logger *log.Logger) {
	if logger != nil && p.logger != nil {
		p.logger.SetOutput(logger.Writer())
		p.logger.SetFlags(logger.Flags())
	}
}

func (p *prometheusOutput) SetEventProcessors(ps map[string]map[string]interface{},
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
				err := ep.Init(
					epCfg[epType],
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

func (p *prometheusOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, p.Cfg)
	if err != nil {
		return err
	}
	if p.Cfg.Name == "" {
		p.Cfg.Name = name
	}
	for _, opt := range opts {
		opt(p)
	}
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

	if p.Cfg.GnmiCache {
		p.gnmiCache = cache.New(
			&cache.GnmiCacheConfig{
				Expiration: p.Cfg.Expiration,
				Timeout:    p.Cfg.Timeout,
				Debug:      p.Cfg.Debug,
			},
			cache.WithLogger(p.logger),
		)
	}

	p.logger.SetPrefix(fmt.Sprintf(loggingPrefix, p.Cfg.Name))
	// create prometheus registry
	registry := prometheus.NewRegistry()

	err = registry.Register(p)
	if err != nil {
		return err
	}
	// create http server
	promHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError})

	mux := http.NewServeMux()
	mux.Handle(p.Cfg.Path, promHandler)

	p.server = &http.Server{
		Addr:    p.Cfg.Listen,
		Handler: mux,
	}

	// create tcp listener
	listener, err := net.Listen("tcp", p.Cfg.Listen)
	if err != nil {
		return err
	}
	// start worker
	p.wg.Add(2)
	wctx, wcancel := context.WithCancel(ctx)
	go p.worker(wctx)
	go p.expireMetricsPeriodic(wctx)
	go func() {
		defer p.wg.Done()
		err = p.server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			p.logger.Printf("prometheus server error: %v", err)
		}
		wcancel()
	}()
	go p.registerService(wctx)
	p.logger.Printf("initialized prometheus output: %s", p.String())
	go func() {
		<-ctx.Done()
		p.Close()
	}()
	return nil
}

// Write implements the outputs.Output interface
func (p *prometheusOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
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
		if p.gnmiCache != nil {
			p.gnmiCache.Write(measName, rsp)
			return
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

func (p *prometheusOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
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

func (p *prometheusOutput) Close() error {
	var err error
	if p.consulClient != nil {
		err = p.consulClient.Agent().ServiceDeregister(p.Cfg.ServiceRegistration.Name)
		if err != nil {
			p.logger.Printf("failed to deregister consul service: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = p.server.Shutdown(ctx)
	if err != nil {
		p.logger.Printf("failed to shutdown http server: %v", err)
	}
	p.logger.Printf("closed.")
	p.wg.Wait()
	return nil
}

func (p *prometheusOutput) RegisterMetrics(reg *prometheus.Registry) {}

// Describe implements prometheus.Collector
func (p *prometheusOutput) Describe(ch chan<- *prometheus.Desc) {}

// Collect implements prometheus.Collector
func (p *prometheusOutput) Collect(ch chan<- prometheus.Metric) {
	p.Lock()
	defer p.Unlock()
	if p.Cfg.GnmiCache {
		p.collectFromCache(ch)
		return
	}
	// No cache
	// run expire before exporting metrics
	p.expireMetrics()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	for _, entry := range p.entries {
		select {
		case <-ctx.Done():
			p.logger.Printf("collection context terminated: %v", ctx.Err())
			return
		case ch <- entry:
		}
	}
}

func (p *prometheusOutput) worker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-p.eventChan:
			if p.Cfg.Debug {
				p.logger.Printf("got event to store: %+v", ev)
			}
			p.Lock()
			for _, pm := range p.metricsFromEvent(ev, time.Now()) {
				key := pm.calculateKey()
				if e, ok := p.entries[key]; ok && pm.time != nil {
					if e.time.Before(*pm.time) {
						p.entries[key] = pm
					}
				} else {
					p.entries[key] = pm
				}
				if p.Cfg.Debug {
					p.logger.Printf("saved key=%d, metric: %+v", key, pm)
				}
			}
			p.Unlock()
		}
	}
}

func (p *prometheusOutput) expireMetrics() {
	if p.Cfg.Expiration <= 0 {
		return
	}
	expiry := time.Now().Add(-p.Cfg.Expiration)
	for k, e := range p.entries {
		if p.Cfg.ExportTimestamps {
			if e.time.Before(expiry) {
				delete(p.entries, k)
			}
			continue
		}
		if e.addedAt.Before(expiry) {
			delete(p.entries, k)
		}
	}
}

func (p *prometheusOutput) expireMetricsPeriodic(ctx context.Context) {
	if p.Cfg.Expiration <= 0 {
		return
	}
	ticker := time.NewTicker(p.Cfg.Expiration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.Lock()
			p.expireMetrics()
			p.Unlock()
		}
	}
}

func (p *prometheusOutput) setDefaults() error {
	if p.Cfg.Listen == "" {
		p.Cfg.Listen = defaultListen
	}
	if p.Cfg.Path == "" {
		p.Cfg.Path = defaultPath
	}
	if p.Cfg.Expiration == 0 {
		p.Cfg.Expiration = defaultExpiration
	}
	if p.Cfg.GnmiCache && p.Cfg.AddTarget == "" {
		p.Cfg.AddTarget = "if-not-present"
	}
	if p.Cfg.Timeout <= 0 {
		p.Cfg.Timeout = defaultTimeout
	}
	p.setServiceRegistrationDefaults()
	var err error
	var port string
	p.Cfg.address, port, err = net.SplitHostPort(p.Cfg.Listen)
	if err != nil {
		p.logger.Printf("invalid 'listen' field format: %v", err)
		return err
	}
	p.Cfg.port, err = strconv.Atoi(port)
	if err != nil {
		p.logger.Printf("invalid 'listen' field format: %v", err)
		return err
	}

	return nil
}

// Metric
func (p *promMetric) calculateKey() uint64 {
	h := fnv.New64a()
	h.Write([]byte(p.name))
	if len(p.labels) > 0 {
		h.Write([]byte(":"))
		sort.Slice(p.labels, func(i, j int) bool {
			return p.labels[i].Name < p.labels[j].Name
		})
		for _, label := range p.labels {
			h.Write([]byte(label.Name))
			h.Write([]byte(":"))
			h.Write([]byte(label.Value))
			h.Write([]byte(":"))
		}
	}
	return h.Sum64()
}

func (p *promMetric) String() string {
	if p == nil {
		return ""
	}
	sb := strings.Builder{}
	sb.WriteString("name=")
	sb.WriteString(p.name)
	sb.WriteString(",")
	numLabels := len(p.labels)
	if numLabels > 0 {
		sb.WriteString("labels=[")
		for i, lb := range p.labels {
			sb.WriteString(lb.Name)
			sb.WriteString("=")
			sb.WriteString(lb.Value)
			if i < numLabels-1 {
				sb.WriteString(",")
			}
		}
		sb.WriteString("],")
	}
	sb.WriteString(fmt.Sprintf("value=%f,", p.value))
	sb.WriteString("time=")
	if p.time != nil {
		sb.WriteString(p.time.String())
	} else {
		sb.WriteString("nil")
	}
	sb.WriteString(",addedAt=")
	sb.WriteString(p.addedAt.String())
	return sb.String()
}

// Desc implements prometheus.Metric
func (p *promMetric) Desc() *prometheus.Desc {
	labelNames := make([]string, 0, len(p.labels))
	for _, label := range p.labels {
		labelNames = append(labelNames, label.Name)
	}

	return prometheus.NewDesc(p.name, defaultMetricHelp, labelNames, nil)
}

// Write implements prometheus.Metric
func (p *promMetric) Write(out *dto.Metric) error {
	out.Untyped = &dto.Untyped{
		Value: &p.value,
	}
	out.Label = make([]*dto.LabelPair, 0, len(p.labels))
	for i := range p.labels {
		out.Label = append(out.Label, &dto.LabelPair{Name: &p.labels[i].Name, Value: &p.labels[i].Value})
	}
	if p.time == nil {
		return nil
	}
	timestamp := p.time.UnixNano() / 1000000
	out.TimestampMs = &timestamp
	return nil
}

func getFloat(v interface{}) (float64, error) {
	switch i := v.(type) {
	case float64:
		return float64(i), nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int16:
		return float64(i), nil
	case int8:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint16:
		return float64(i), nil
	case uint8:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case string:
		f, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return math.NaN(), err
		}
		return f, err
	case *gnmi.Decimal64:
		return float64(i.Digits) / math.Pow10(int(i.Precision)), nil
	default:
		return math.NaN(), errors.New("getFloat: unknown value is of incompatible type")
	}
}

func (p *prometheusOutput) SetName(name string) {
	if p.Cfg.Name == "" {
		p.Cfg.Name = name
	}
	if p.Cfg.ServiceRegistration != nil {
		if p.Cfg.ServiceRegistration.Name == "" {
			p.Cfg.ServiceRegistration.Name = fmt.Sprintf("prometheus-%s", p.Cfg.Name)
		}
		if name == "" {
			name = uuid.New().String()
		}
		p.Cfg.ServiceRegistration.id = fmt.Sprintf("%s-%s", p.Cfg.ServiceRegistration.Name, name)
		p.Cfg.ServiceRegistration.Tags = append(p.Cfg.ServiceRegistration.Tags, fmt.Sprintf("gnmic-instance=%s", name))
	}
}

func (p *prometheusOutput) SetClusterName(name string) {
	p.Cfg.clusterName = name
	if p.Cfg.ServiceRegistration != nil {
		p.Cfg.ServiceRegistration.Tags = append(p.Cfg.ServiceRegistration.Tags, fmt.Sprintf("gnmic-cluster=%s", name))
	}
}

func (p *prometheusOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}

func (p *prometheusOutput) metricsFromEvent(ev *formatters.EventMsg, now time.Time) []*promMetric {
	pms := make([]*promMetric, 0, len(ev.Values))
	labels := p.mb.GetLabels(ev)
	for vName, val := range ev.Values {
		v, err := getFloat(val)
		if err != nil {
			if !p.Cfg.StringsAsLabels {
				continue
			}
			v = 1.0
		}
		pm := &promMetric{
			name:    p.mb.MetricName(ev.Name, vName),
			labels:  labels,
			value:   v,
			addedAt: now,
		}
		if p.Cfg.OverrideTimestamps && p.Cfg.ExportTimestamps {
			ev.Timestamp = now.UnixNano()
		}
		if p.Cfg.ExportTimestamps {
			tm := time.Unix(0, ev.Timestamp)
			pm.time = &tm
		}
		pms = append(pms, pm)
	}
	return pms
}
