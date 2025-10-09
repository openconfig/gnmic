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
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	promcom "github.com/openconfig/gnmic/pkg/outputs/prometheus_output"
	gutils "github.com/openconfig/gnmic/pkg/utils"
)

const (
	outputType        = "prometheus"
	defaultListen     = ":9804"
	defaultPath       = "/metrics"
	defaultExpiration = time.Minute
	loggingPrefix     = "[prometheus_output:%s] "
	// this is used to timeout the collection method
	// in case it drags for too long
	defaultTimeout    = 10 * time.Second
	defaultNumWorkers = 1
)

func init() {
	outputs.Register(outputType, func() outputs.Output {
		return &prometheusOutput{
			cfg:       &config{},
			eventChan: make(chan *formatters.EventMsg),
			msgChan:   make(chan *outputs.ProtoMsg, 10_000),
			wg:        new(sync.WaitGroup),
			entries:   make(map[uint64]*promcom.PromMetric),
			logger:    log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type prometheusOutput struct {
	outputs.BaseOutput
	cfg       *config
	logger    *log.Logger
	eventChan chan *formatters.EventMsg
	msgChan   chan *outputs.ProtoMsg

	wg     *sync.WaitGroup
	server *http.Server
	sync.Mutex
	entries map[uint64]*promcom.PromMetric

	mb           *promcom.MetricBuilder
	evps         []formatters.EventProcessor
	consulClient *api.Client

	targetTpl *template.Template

	gnmiCache   cache.Cache
	targetsMeta *ttlcache.Cache[string, outputs.Meta]

	reg   *prometheus.Registry
	store store.Store[any]
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

func (p *prometheusOutput) setEventProcessors(logger *log.Logger) error {
	tcs, ps, acts, err := gutils.GetConfigMaps(p.store)
	if err != nil {
		return err
	}
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

func (p *prometheusOutput) setLogger(logger *log.Logger) {
	if logger != nil && p.logger != nil {
		p.logger.SetOutput(logger.Writer())
		p.logger.SetFlags(logger.Flags())
	}
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

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	p.store = options.Store

	// apply logger
	p.setLogger(options.Logger)

	// initialize target template
	if p.cfg.TargetTemplate == "" {
		p.targetTpl = outputs.DefaultTargetTemplate
	} else if p.cfg.AddTarget != "" {
		p.targetTpl, err = gtemplate.CreateTemplate("target-template", p.cfg.TargetTemplate)
		if err != nil {
			return err
		}
		p.targetTpl = p.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	// set defaults
	err = p.setDefaults()
	if err != nil {
		return err
	}

	// initialize registry
	p.reg = options.Registry
	err = p.registerMetrics()
	if err != nil {
		return err
	}
	p.setName(options.Name)
	p.setClusterName(options.ClusterName)
	// initialize event processors
	err = p.setEventProcessors(options.Logger)
	if err != nil {
		return err
	}
	p.mb = &promcom.MetricBuilder{
		Prefix:                 p.cfg.MetricPrefix,
		AppendSubscriptionName: p.cfg.AppendSubscriptionName,
		StringsAsLabels:        p.cfg.StringsAsLabels,
		OverrideTimestamps:     p.cfg.OverrideTimestamps,
		ExportTimestamps:       p.cfg.ExportTimestamps,
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
	switch p.cfg.TLS {
	case nil:
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
		p.workerHandleEvent(events...)
	}
}

type metricAndKey struct {
	k uint64
	m *promcom.PromMetric
}

func (p *prometheusOutput) workerHandleEvent(evs ...*formatters.EventMsg) {
	if p.cfg.Debug {
		p.logger.Printf("got event to store: %+v", evs)
	}
	mks := make([]*metricAndKey, 0, len(evs))
	for _, ev := range evs {
		for _, pm := range p.mb.MetricsFromEvent(ev, time.Now()) {
			mks = append(mks, &metricAndKey{
				m: pm,
				k: pm.CalculateKey(),
			})
		}
	}
	p.Lock()

	defer p.Unlock()
	for _, mk := range mks {
		//	key := pm.CalculateKey()
		e, ok := p.entries[mk.k]
		// if the entry key is not present add it to the map.
		// if present add it only if the entry timestamp is newer than the
		// existing one.
		if !ok || mk.m.Time == nil || (ok && mk.m.Time != nil && e.Time.Before(*mk.m.Time)) {
			p.entries[mk.k] = mk.m
			if p.cfg.Debug {
				p.logger.Printf("saved key=%d, metric: %+v", mk.k, mk.m)
			}
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
			if e.Time.Before(expiry) {
				delete(p.entries, k)
			}
			continue
		}
		if e.AddedAt.Before(expiry) {
			delete(p.entries, k)
		}
	}
}

func (p *prometheusOutput) expireMetricsPeriodic(ctx context.Context) {
	if p.cfg.Expiration <= 0 {
		return
	}
	p.Lock()
	prometheusNumberOfMetrics.WithLabelValues(p.cfg.Name).Set(float64(len(p.entries)))
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
			prometheusNumberOfMetrics.WithLabelValues(p.cfg.Name).Set(float64(len(p.entries)))
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

func (p *prometheusOutput) setName(name string) {
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

func (p *prometheusOutput) setClusterName(name string) {
	p.cfg.clusterName = name
	if p.cfg.ServiceRegistration != nil {
		p.cfg.ServiceRegistration.Tags = append(p.cfg.ServiceRegistration.Tags, fmt.Sprintf("gnmic-cluster=%s", name))
	}
}
