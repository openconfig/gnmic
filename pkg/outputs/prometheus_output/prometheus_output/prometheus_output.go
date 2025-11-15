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
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	promcom "github.com/openconfig/gnmic/pkg/outputs/prometheus_output"
	"github.com/openconfig/gnmic/pkg/store"
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
		return &prometheusOutput{}
	})
}

type prometheusOutput struct {
	outputs.BaseOutput

	cfg       *atomic.Pointer[config]
	dynCfg    *atomic.Pointer[dynConfig]
	logger    *log.Logger
	eventChan chan *formatters.EventMsg
	msgChan   chan *outputs.ProtoMsg

	wg     *sync.WaitGroup
	server *http.Server

	sync.Mutex
	entries map[uint64]*promcom.PromMetric

	consulClient *api.Client

	gnmiCache   cache.Cache
	targetsMeta *ttlcache.Cache[string, outputs.Meta]

	reg    *prometheus.Registry
	store  store.Store[any]
	runCfn context.CancelFunc
	runCtx context.Context
}

type dynConfig struct {
	targetTpl *template.Template
	evps      []formatters.EventProcessor
	mb        *promcom.MetricBuilder
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
	cfg := p.cfg.Load()
	if cfg == nil {
		return ""
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (p *prometheusOutput) buildEventProcessors(cfg *config) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(p.store)
	if err != nil {
		return nil, err
	}
	return formatters.MakeEventProcessors(p.logger, cfg.EventProcessors, ps, tcs, acts)
}

func (p *prometheusOutput) setLogger(logger *log.Logger) {
	if logger != nil && p.logger != nil {
		p.logger.SetOutput(logger.Writer())
		p.logger.SetFlags(logger.Flags())
	}
}

func (p *prometheusOutput) init() {
	p.cfg = new(atomic.Pointer[config])
	p.dynCfg = new(atomic.Pointer[dynConfig])
	p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	p.eventChan = make(chan *formatters.EventMsg)
	p.msgChan = make(chan *outputs.ProtoMsg, 10_000)
	p.wg = new(sync.WaitGroup)
	p.entries = make(map[uint64]*promcom.PromMetric)
}

func (p *prometheusOutput) Init(ctx context.Context, name string, cfg map[string]any, opts ...outputs.Option) error {
	p.init() // init struct fields
	p.runCtx, p.runCfn = context.WithCancel(ctx)
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	if newCfg.Name == "" {
		newCfg.Name = name
	}

	p.logger.SetPrefix(fmt.Sprintf(loggingPrefix, newCfg.Name))

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
	err = p.setDefaultsFor(newCfg)
	if err != nil {
		return err
	}

	// initialize registry
	p.reg = options.Registry
	err = p.registerMetrics()
	if err != nil {
		return err
	}
	p.setName(options.Name, newCfg)
	p.setClusterName(options.ClusterName, newCfg)

	p.cfg.Store(newCfg)

	dc := new(dynConfig)

	// initialize target template
	if newCfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}
	// initialize event processors
	dc.evps, err = p.buildEventProcessors(newCfg)
	if err != nil {
		return err
	}

	dc.mb = &promcom.MetricBuilder{
		Prefix:                 newCfg.MetricPrefix,
		AppendSubscriptionName: newCfg.AppendSubscriptionName,
		StringsAsLabels:        newCfg.StringsAsLabels,
		OverrideTimestamps:     newCfg.OverrideTimestamps,
		ExportTimestamps:       newCfg.ExportTimestamps,
	}

	p.dynCfg.Store(dc)

	if newCfg.CacheConfig != nil {
		p.gnmiCache, err = cache.New(
			newCfg.CacheConfig,
			cache.WithLogger(p.logger),
		)
		if err != nil {
			return err
		}
		p.targetsMeta = ttlcache.New(ttlcache.WithTTL[string, outputs.Meta](newCfg.Expiration))
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
	mux.Handle(newCfg.Path, promHandler)

	p.server = &http.Server{
		Addr:    newCfg.Listen,
		Handler: mux,
	}

	// create tcp listener
	listener, err := p.createListenerFor(newCfg)
	if err != nil {
		return err
	}

	// start worker
	p.wg.Add(newCfg.NumWorkers)
	for i := 0; i < newCfg.NumWorkers; i++ {
		go p.worker(p.runCtx)
	}

	if newCfg.CacheConfig == nil {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.expireMetricsPeriodic(p.runCtx)
		}()
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer listener.Close()
		err = p.server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			p.logger.Printf("prometheus server error: %v", err)
		}
	}()
	go p.registerService(p.runCtx)
	p.logger.Printf("initialized prometheus output: %s", p.String())
	return nil
}

func (p *prometheusOutput) Validate(cfg map[string]any) error {
	ncfg := new(config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}
	err = p.setDefaultsFor(ncfg)
	if err != nil {
		return err
	}
	_, err = gtemplate.CreateTemplate("target-template", ncfg.TargetTemplate)
	if err != nil {
		return err
	}
	_, err = gtemplate.CreateTemplate("target-template", ncfg.TargetTemplate)
	if err != nil {
		return err
	}
	return nil
}

func (p *prometheusOutput) Update(ctx context.Context, cfg map[string]any) error {
	// decode new config
	newCfg := new(config)
	if err := outputs.DecodeConfig(cfg, newCfg); err != nil {
		return err
	}

	currCfg := p.cfg.Load()
	dc := new(dynConfig)
	// apply defaults and derived fields for the new config
	tmp := *newCfg    // copy for mutation
	if p.cfg != nil { // init name and service registration name, id and tags
		tmp.Name = currCfg.Name
		if currCfg.ServiceRegistration != nil {
			if tmp.ServiceRegistration.Name == "" {
				tmp.ServiceRegistration.Name = currCfg.ServiceRegistration.Name
			}
			tmp.ServiceRegistration.id = fmt.Sprintf("%s-%s", tmp.ServiceRegistration.Name, tmp.Name)
			tmp.ServiceRegistration.Tags = append(tmp.ServiceRegistration.Tags, fmt.Sprintf("gnmic-instance=%s", tmp.ServiceRegistration.Name))
		}
	}
	if err := p.setDefaultsFor(&tmp); err != nil { // factor setDefaults to accept *config
		return err
	}

	// rebuild objects that depend on config
	dc.targetTpl = outputs.DefaultTargetTemplate
	if tmp.TargetTemplate != "" {
		t, err := gtemplate.CreateTemplate("target-template", tmp.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = t.Funcs(outputs.TemplateFuncs)
	}

	// event processors
	var err error
	prevDC := p.dynCfg.Load()
	if slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0 {
		dc.evps, err = p.buildEventProcessors(&tmp)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}

	// metric builder
	dc.mb = &promcom.MetricBuilder{
		Prefix:                 tmp.MetricPrefix,
		AppendSubscriptionName: tmp.AppendSubscriptionName,
		StringsAsLabels:        tmp.StringsAsLabels,
		OverrideTimestamps:     tmp.OverrideTimestamps,
		ExportTimestamps:       tmp.ExportTimestamps,
	}

	p.dynCfg.Store(dc)

	// rebuild http objects if needed
	rebuildHTTPServer := p.needHTTPRebuild(currCfg, &tmp)
	var newServer *http.Server
	var newListener net.Listener
	if rebuildHTTPServer {
		reg := prometheus.NewRegistry()
		if err := reg.Register(p); err != nil {
			return err
		}
		promHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError})
		mux := http.NewServeMux()
		mux.Handle(tmp.Path, promHandler)

		s := &http.Server{
			Addr:    tmp.Listen,
			Handler: mux,
		}
		l, err := p.createListenerFor(&tmp)
		if err != nil {
			return err
		}
		newServer = s
		newListener = l
	}

	// cache rebuild if CacheConfig toggled or changed
	var newCache cache.Cache
	var newTargetsMeta *ttlcache.Cache[string, outputs.Meta]
	if !cacheEqual(currCfg.CacheConfig, tmp.CacheConfig) {
		if tmp.CacheConfig != nil {
			c, err := cache.New(tmp.CacheConfig, cache.WithLogger(p.logger))
			if err != nil {
				return err
			}
			newCache = c
			newTargetsMeta = ttlcache.New(ttlcache.WithTTL[string, outputs.Meta](tmp.Expiration))
		}
	} else {
		// keep existing cache/meta if not changed
		p.Lock()
		newCache = p.gnmiCache
		newTargetsMeta = p.targetsMeta
		p.Unlock()
	}

	// swap under lock
	p.Lock()
	oldServer := p.server
	oldRunCfn := p.runCfn
	oldCache := p.gnmiCache

	p.cfg.Store(&tmp)

	if rebuildHTTPServer {
		p.server = newServer
	}
	if newCache != nil || (oldCache != nil && tmp.CacheConfig == nil) {
		p.gnmiCache = newCache
		p.targetsMeta = newTargetsMeta
	}
	// create a new worker ctx
	p.runCtx, p.runCfn = context.WithCancel(ctx)
	p.Unlock()

	// Start/Restart components that changed

	// HTTP server
	if rebuildHTTPServer {
		if oldServer != nil {
			_ = oldServer.Close() // stop old server; Serve will exit
		}
		// start the new one
		p.wg.Add(1)
		go func(srv *http.Server, l net.Listener) {
			defer p.wg.Done()
			defer l.Close()
			if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
				p.logger.Printf("prometheus server error: %v", err)
			}
		}(newServer, newListener)
	}

	// workers (stop old, start new)
	if oldRunCfn != nil {
		oldRunCfn()
	}
	// start workers with new num-workers
	p.wg.Add(tmp.NumWorkers)
	for i := 0; i < tmp.NumWorkers; i++ {
		go p.worker(p.runCtx)
	}
	if tmp.CacheConfig == nil {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.expireMetricsPeriodic(p.runCtx)
		}()
	}

	// restart service registration
	go p.registerService(p.runCtx)

	p.logger.Printf("updated prometheus output: %s", p.String())
	return nil
}

func (p *prometheusOutput) needHTTPRebuild(old, new *config) bool {
	if p.server == nil || old == nil || new == nil {
		return true
	}
	return old.Listen != new.Listen ||
		old.Path != new.Path ||
		!old.TLS.Equal(new.TLS)
}

func (p *prometheusOutput) createListenerFor(c *config) (net.Listener, error) {
	if c.TLS == nil {
		return net.Listen("tcp", c.Listen)
	}
	tlsConfig, err := utils.NewTLSConfig(
		c.TLS.CaFile, c.TLS.CertFile, c.TLS.KeyFile, c.TLS.ClientAuth, true, true,
	)
	if err != nil {
		return nil, err
	}
	return tls.Listen("tcp", c.Listen, tlsConfig)
}

// Write implements the outputs.Output interface
func (p *prometheusOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	cfg := p.cfg.Load()
	if cfg == nil {
		return
	}

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

func (p *prometheusOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
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

func (p *prometheusOutput) Close() error {
	p.Lock()
	consulClient := p.consulClient
	gnmiCache := p.gnmiCache
	server := p.server
	cfg := p.cfg.Load()
	p.Unlock()

	var err error
	if consulClient != nil && cfg != nil && cfg.ServiceRegistration != nil {
		err = consulClient.Agent().ServiceDeregister(cfg.ServiceRegistration.id)
		if err != nil {
			// ignore 404 and unknown service ID errors
			if !strings.Contains(err.Error(), "404") &&
				!strings.Contains(err.Error(), "Unknown service ID") {
				p.logger.Printf("failed to deregister consul service: %v", err)
			}
		}
	}
	if p.runCfn != nil {
		p.runCfn()
	}
	if gnmiCache != nil {
		gnmiCache.Stop()
	}
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = server.Shutdown(ctx)
		if err != nil {
			p.logger.Printf("failed to shutdown http server: %v", err)
		}
	}
	p.logger.Printf("closed.")
	p.wg.Wait()
	return nil
}

// Describe implements prometheus.Collector
func (p *prometheusOutput) Describe(ch chan<- *prometheus.Desc) {}

// Collect implements prometheus.Collector
func (p *prometheusOutput) Collect(ch chan<- prometheus.Metric) {
	cfg := p.cfg.Load()
	if cfg == nil {
		return
	}

	p.Lock()
	defer p.Unlock()
	if cfg.CacheConfig != nil {
		p.collectFromCache(ch)
		return
	}
	// No cache
	// run expire before exporting metrics
	p.expireMetrics()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
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
	cfg := p.cfg.Load()
	dc := p.dynCfg.Load()

	if cfg == nil || dc == nil {
		return
	}
	p.Lock()
	gnmiCache := p.gnmiCache
	targetsMeta := p.targetsMeta
	p.Unlock()

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
		if gnmiCache != nil {
			gnmiCache.Write(ctx, measName, pmsg)
			target := utils.GetHost(meta["source"])
			targetsMeta.Set(measName+"/"+target, meta, ttlcache.DefaultTTL)
			return
		}
		events, err := formatters.ResponseToEventMsgs(measName, pmsg, meta, dc.evps...)
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
	cfg := p.cfg.Load()
	dc := p.dynCfg.Load()
	if cfg == nil || dc == nil {
		return
	}
	if cfg.Debug {
		p.logger.Printf("got event to store: %+v", evs)
	}
	mks := make([]*metricAndKey, 0, len(evs))
	for _, ev := range evs {
		for _, pm := range dc.mb.MetricsFromEvent(ev, time.Now()) {
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
			if cfg.Debug {
				p.logger.Printf("saved key=%d, metric: %+v", mk.k, mk.m)
			}
		}
	}
}

func (p *prometheusOutput) expireMetrics() {
	cfg := p.cfg.Load()
	if cfg == nil || cfg.Expiration <= 0 {
		return
	}
	expiry := time.Now().Add(-cfg.Expiration)
	for k, e := range p.entries {
		if cfg.ExportTimestamps {
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
	cfg := p.cfg.Load()
	if cfg == nil || cfg.Expiration <= 0 {
		return
	}

	p.Lock()
	prometheusNumberOfMetrics.WithLabelValues(cfg.Name).Set(float64(len(p.entries)))
	p.Unlock()

	ticker := time.NewTicker(cfg.Expiration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cfg := p.cfg.Load()
			if cfg == nil {
				continue
			}
			p.Lock()
			p.expireMetrics()
			prometheusNumberOfMetrics.WithLabelValues(cfg.Name).Set(float64(len(p.entries)))
			p.Unlock()
		}
	}
}

func (p *prometheusOutput) setDefaultsFor(c *config) error {
	if c.Listen == "" {
		c.Listen = defaultListen
	}
	if c.Path == "" {
		c.Path = defaultPath
	}
	if c.Expiration == 0 {
		c.Expiration = defaultExpiration
	}
	if c.CacheConfig != nil && c.AddTarget == "" {
		c.AddTarget = "if-not-present"
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.NumWorkers <= 0 {
		c.NumWorkers = defaultNumWorkers
	}
	if c.ServiceRegistration == nil {
		return nil
	}

	p.setServiceRegistrationDefaults(c)
	var err error
	var port string
	switch {
	case c.ServiceRegistration.ServiceAddress != "":
		c.address, port, err = net.SplitHostPort(c.ServiceRegistration.ServiceAddress)
		if err != nil {
			// if service-address does not include a port number, use the port number from the listen field
			if strings.Contains(err.Error(), "missing port in address") {
				c.address = c.ServiceRegistration.ServiceAddress
				_, port, err = net.SplitHostPort(c.Listen)
				if err != nil {
					p.logger.Printf("invalid 'listen' field format: %v", err)
					return err
				}
				c.port, err = strconv.Atoi(port)
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
		c.port, err = strconv.Atoi(port)
		if err != nil {
			p.logger.Printf("invalid 'service-registration.service-address' field format: %v", err)
			return err
		}
	default:
		c.address, port, err = net.SplitHostPort(c.Listen)
		if err != nil {
			p.logger.Printf("invalid 'listen' field format: %v", err)
			return err
		}
		c.port, err = strconv.Atoi(port)
		if err != nil {
			p.logger.Printf("invalid 'listen' field format: %v", err)
			return err
		}
	}

	return nil
}

func (p *prometheusOutput) setName(name string, cfg *config) {
	if cfg.Name == "" {
		cfg.Name = name
	}
	if cfg.ServiceRegistration != nil {
		if cfg.ServiceRegistration.Name == "" {
			cfg.ServiceRegistration.Name = fmt.Sprintf("prometheus-%s", cfg.Name)
		}
		if name == "" {
			name = uuid.New().String()
		}
		cfg.ServiceRegistration.id = fmt.Sprintf("%s-%s", cfg.ServiceRegistration.Name, name)
		cfg.ServiceRegistration.Tags = append(cfg.ServiceRegistration.Tags, fmt.Sprintf("gnmic-instance=%s", name))
	}
}

func (p *prometheusOutput) setClusterName(name string, cfg *config) {
	cfg.clusterName = name
	if cfg.ServiceRegistration != nil {
		cfg.ServiceRegistration.Tags = append(cfg.ServiceRegistration.Tags, fmt.Sprintf("gnmic-cluster=%s", name))
	}
}
