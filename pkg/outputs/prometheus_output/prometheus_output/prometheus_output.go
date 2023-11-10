// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_output

import (
	"context"
	"crypto/tls"
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
	"github.com/jellydator/ttlcache/v3"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	promcom "github.com/openconfig/gnmic/pkg/outputs/prometheus_output"
	"github.com/openconfig/gnmic/pkg/types"
	"github.com/openconfig/gnmic/pkg/utils"
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
	defaultTimeout    = 10 * time.Second
	defaultNumWorkers = 1
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
			cfg:       &config{},
			eventChan: make(chan *formatters.EventMsg),
			msgChan:   make(chan *outputs.ProtoMsg),
			wg:        new(sync.WaitGroup),
			entries:   make(map[uint64]*promMetric),
			logger:    log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type prometheusOutput struct {
	cfg       *config
	logger    *log.Logger
	eventChan chan *formatters.EventMsg
	msgChan   chan *outputs.ProtoMsg

	wg     *sync.WaitGroup
	server *http.Server
	sync.Mutex
	entries map[uint64]*promMetric

	mb           *promcom.MetricBuilder
	evps         []formatters.EventProcessor
	consulClient *api.Client

	targetTpl *template.Template

	gnmiCache   cache.Cache
	targetsMeta *ttlcache.Cache[string, outputs.Meta]
}

type config struct {
	Name                   string               `mapstructure:"name,omitempty" json:"name,omitempty"`
	Listen                 string               `mapstructure:"listen,omitempty" json:"listen,omitempty"`
	TLS                    *types.TLSConfig     `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	Path                   string               `mapstructure:"path,omitempty" json:"path,omitempty"`
	Expiration             time.Duration        `mapstructure:"expiration,omitempty" json:"expiration,omitempty"`
	MetricPrefix           string               `mapstructure:"metric-prefix,omitempty" json:"metric-prefix,omitempty"`
	AppendSubscriptionName bool                 `mapstructure:"append-subscription-name,omitempty" json:"append-subscription-name,omitempty"`
	ExportTimestamps       bool                 `mapstructure:"export-timestamps,omitempty" json:"export-timestamps,omitempty"`
	OverrideTimestamps     bool                 `mapstructure:"override-timestamps,omitempty" json:"override-timestamps,omitempty"`
	AddTarget              string               `mapstructure:"add-target,omitempty" json:"add-target,omitempty"`
	TargetTemplate         string               `mapstructure:"target-template,omitempty" json:"target-template,omitempty"`
	StringsAsLabels        bool                 `mapstructure:"strings-as-labels,omitempty" json:"strings-as-labels,omitempty"`
	Debug                  bool                 `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	EventProcessors        []string             `mapstructure:"event-processors,omitempty" json:"event-processors,omitempty"`
	ServiceRegistration    *serviceRegistration `mapstructure:"service-registration,omitempty" json:"service-registration,omitempty"`
	Timeout                time.Duration        `mapstructure:"timeout,omitempty" json:"timeout,omitempty"`
	CacheConfig            *cache.Config        `mapstructure:"cache,omitempty" json:"cache-config,omitempty"`
	NumWorkers             int                  `mapstructure:"num-workers,omitempty" json:"num-workers,omitempty"`
	EnableMetrics          bool                 `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`

	clusterName string
	address     string
	port        int
}

func (p *prometheusOutput) String() string {
	b, err := json.Marshal(p.cfg)
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
	acts map[string]map[string]interface{}) error {
	var err error
	p.evps, err = outputs.MakeEventProcessors(
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

func (p *prometheusOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, p.cfg)
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

	if p.cfg.CacheConfig != nil {
		p.gnmiCache, err = cache.New(
			p.cfg.CacheConfig,
			cache.WithLogger(p.logger),
		)
		if err != nil {
			return err
		}
		p.targetsMeta = ttlcache.New(ttlcache.WithTTL[string, outputs.Meta](p.cfg.Expiration))
	}

	// create prometheus registry
	registry := prometheus.NewRegistry()

	err = registry.Register(p)
	if err != nil {
		return err
	}
	// create http server
	promHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError})

	mux := http.NewServeMux()
	mux.Handle(p.cfg.Path, promHandler)

	p.server = &http.Server{
		Addr:    p.cfg.Listen,
		Handler: mux,
	}

	// create tcp listener
	var listener net.Listener
	switch {
	case p.cfg.TLS == nil:
		listener, err = net.Listen("tcp", p.cfg.Listen)
	default:
		var tlsConfig *tls.Config
		tlsConfig, err = utils.NewTLSConfig(
			p.cfg.TLS.CaFile,
			p.cfg.TLS.CertFile,
			p.cfg.TLS.KeyFile,
			p.cfg.TLS.ClientAuth,
			true,
			true,
		)
		if err != nil {
			return err
		}
		listener, err = tls.Listen("tcp", p.cfg.Listen, tlsConfig)
	}
	if err != nil {
		return err
	}
	// start worker
	p.wg.Add(1 + p.cfg.NumWorkers)
	wctx, wcancel := context.WithCancel(ctx)
	for i := 0; i < p.cfg.NumWorkers; i++ {
		go p.worker(wctx)
	}

	if p.cfg.CacheConfig == nil {
		go p.expireMetricsPeriodic(wctx)
	}

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
		err = p.consulClient.Agent().ServiceDeregister(p.cfg.ServiceRegistration.Name)
		if err != nil {
			p.logger.Printf("failed to deregister consul service: %v", err)
		}
	}
	if p.gnmiCache != nil {
		p.gnmiCache.Stop()
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

func (p *prometheusOutput) RegisterMetrics(reg *prometheus.Registry) {
	if !p.cfg.EnableMetrics {
		return
	}
	if err := p.registerMetrics(reg); err != nil {
		p.logger.Printf("failed to register metric: %v", err)
	}
}

// Describe implements prometheus.Collector
func (p *prometheusOutput) Describe(ch chan<- *prometheus.Desc) {}

// Collect implements prometheus.Collector
func (p *prometheusOutput) Collect(ch chan<- prometheus.Metric) {
	p.Lock()
	defer p.Unlock()
	if p.cfg.CacheConfig != nil {
		p.collectFromCache(ch)
		return
	}
	// No cache
	// run expire before exporting metrics
	p.expireMetrics()

	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.Timeout)
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
			p.workerHandleEvent(ev)
		case m := <-p.msgChan:
			p.workerHandleProto(ctx, m)
		}
	}
}

func (p *prometheusOutput) workerHandleProto(ctx context.Context, m *outputs.ProtoMsg) {
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
		if p.gnmiCache != nil {
			p.gnmiCache.Write(ctx, measName, pmsg)
			target := utils.GetHost(meta["source"])
			p.targetsMeta.Set(measName+"/"+target, meta, ttlcache.DefaultTTL)
			return
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

func (p *prometheusOutput) workerHandleEvent(ev *formatters.EventMsg) {
	if p.cfg.Debug {
		p.logger.Printf("got event to store: %+v", ev)
	}
	p.Lock()
	defer p.Unlock()
	for _, pm := range p.metricsFromEvent(ev, time.Now()) {
		key := pm.calculateKey()
		e, ok := p.entries[key]
		// if the entry key is not present add it to the map.
		// if present add it only if the entry timestamp is newer than the
		// existing one.
		if !ok || (ok && pm.time != nil && e.time.Before(*pm.time)) {
			p.entries[key] = pm
		}
		if p.cfg.Debug {
			p.logger.Printf("saved key=%d, metric: %+v", key, pm)
		}
	}
}

func (p *prometheusOutput) expireMetrics() {
	if p.cfg.Expiration <= 0 {
		return
	}
	expiry := time.Now().Add(-p.cfg.Expiration)
	for k, e := range p.entries {
		if p.cfg.ExportTimestamps {
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
	if p.cfg.Expiration <= 0 {
		return
	}
	p.Lock()
	prometheusNumberOfMetrics.Set(float64(len(p.entries)))
	p.Unlock()
	ticker := time.NewTicker(p.cfg.Expiration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.Lock()
			p.expireMetrics()
			prometheusNumberOfMetrics.Set(float64(len(p.entries)))
			p.Unlock()
		}
	}
}

func (p *prometheusOutput) setDefaults() error {
	if p.cfg.Listen == "" {
		p.cfg.Listen = defaultListen
	}
	if p.cfg.Path == "" {
		p.cfg.Path = defaultPath
	}
	if p.cfg.Expiration == 0 {
		p.cfg.Expiration = defaultExpiration
	}
	if p.cfg.CacheConfig != nil && p.cfg.AddTarget == "" {
		p.cfg.AddTarget = "if-not-present"
	}
	if p.cfg.Timeout <= 0 {
		p.cfg.Timeout = defaultTimeout
	}
	if p.cfg.NumWorkers <= 0 {
		p.cfg.NumWorkers = defaultNumWorkers
	}
	if p.cfg.ServiceRegistration == nil {
		return nil
	}

	p.setServiceRegistrationDefaults()
	var err error
	var port string
	switch {
	case p.cfg.ServiceRegistration.ServiceAddress != "":
		p.cfg.address, port, err = net.SplitHostPort(p.cfg.ServiceRegistration.ServiceAddress)
		if err != nil {
			// if service-address does not include a port number, use the port number from the listen field
			if strings.Contains(err.Error(), "missing port in address") {
				p.cfg.address = p.cfg.ServiceRegistration.ServiceAddress
				_, port, err = net.SplitHostPort(p.cfg.Listen)
				if err != nil {
					p.logger.Printf("invalid 'listen' field format: %v", err)
					return err
				}
				p.cfg.port, err = strconv.Atoi(port)
				if err != nil {
					p.logger.Printf("invalid 'listen' field format: %v", err)
					return err
				}
				return nil
			}
			// if the error is not related to a missing port, fail
			p.logger.Printf("invalid 'service-registration.service-address' field format: %v", err)
			return err
		}
		// the service-address contains both an address and a port number
		p.cfg.port, err = strconv.Atoi(port)
		if err != nil {
			p.logger.Printf("invalid 'service-registration.service-address' field format: %v", err)
			return err
		}
	default:
		p.cfg.address, port, err = net.SplitHostPort(p.cfg.Listen)
		if err != nil {
			p.logger.Printf("invalid 'listen' field format: %v", err)
			return err
		}
		p.cfg.port, err = strconv.Atoi(port)
		if err != nil {
			p.logger.Printf("invalid 'listen' field format: %v", err)
			return err
		}
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
		//lint:ignore SA1019 still need DecimalVal for backward compatibility
	case *gnmi.Decimal64:
		return float64(i.Digits) / math.Pow10(int(i.Precision)), nil
	default:
		return math.NaN(), errors.New("getFloat: unknown value is of incompatible type")
	}
}

func (p *prometheusOutput) SetName(name string) {
	if p.cfg.Name == "" {
		p.cfg.Name = name
	}
	if p.cfg.ServiceRegistration != nil {
		if p.cfg.ServiceRegistration.Name == "" {
			p.cfg.ServiceRegistration.Name = fmt.Sprintf("prometheus-%s", p.cfg.Name)
		}
		if name == "" {
			name = uuid.New().String()
		}
		p.cfg.ServiceRegistration.id = fmt.Sprintf("%s-%s", p.cfg.ServiceRegistration.Name, name)
		p.cfg.ServiceRegistration.Tags = append(p.cfg.ServiceRegistration.Tags, fmt.Sprintf("gnmic-instance=%s", name))
	}
}

func (p *prometheusOutput) SetClusterName(name string) {
	p.cfg.clusterName = name
	if p.cfg.ServiceRegistration != nil {
		p.cfg.ServiceRegistration.Tags = append(p.cfg.ServiceRegistration.Tags, fmt.Sprintf("gnmic-cluster=%s", name))
	}
}

func (p *prometheusOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}

func (p *prometheusOutput) metricsFromEvent(ev *formatters.EventMsg, now time.Time) []*promMetric {
	pms := make([]*promMetric, 0, len(ev.Values))
	labels := p.mb.GetLabels(ev)
	for vName, val := range ev.Values {
		v, err := getFloat(val)
		if err != nil {
			if !p.cfg.StringsAsLabels {
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
		if p.cfg.OverrideTimestamps && p.cfg.ExportTimestamps {
			ev.Timestamp = now.UnixNano()
		}
		if p.cfg.ExportTimestamps {
			tm := time.Unix(0, ev.Timestamp)
			pm.time = &tm
		}
		pms = append(pms, pm)
	}
	return pms
}
