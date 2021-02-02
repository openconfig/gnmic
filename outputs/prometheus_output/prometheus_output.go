package prometheus_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

const (
	defaultListen                     = ":9804"
	defaultPath                       = "/metrics"
	defaultExpiration                 = time.Minute
	defaultMetricHelp                 = "gNMIc generated metric"
	defaultServiceRegistrationAddress = "localhost:8500"
	defaultRegistrationCheckInterval  = 5 * time.Second
	defaultMaxServiceFail             = 3
	metricNameRegex                   = "[^a-zA-Z0-9_]+"
	loggingPrefix                     = "[prometheus_output] "
)

type labelPair struct {
	Name  string
	Value string
}
type promMetric struct {
	name   string
	labels []*labelPair
	time   *time.Time
	value  float64
	// addedAt is used to expire metrics if the time field is not initialized
	// this happens when ExportTimestamp == false
	addedAt time.Time
}

func init() {
	outputs.Register("prometheus", func() outputs.Output {
		return &PrometheusOutput{
			Cfg:         &Config{},
			eventChan:   make(chan *formatters.EventMsg),
			wg:          new(sync.WaitGroup),
			entries:     make(map[uint64]*promMetric),
			metricRegex: regexp.MustCompile(metricNameRegex),
			logger:      log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

type PrometheusOutput struct {
	Cfg       *Config
	logger    *log.Logger
	eventChan chan *formatters.EventMsg

	wg     *sync.WaitGroup
	server *http.Server
	sync.Mutex
	entries map[uint64]*promMetric

	metricRegex  *regexp.Regexp
	evps         []formatters.EventProcessor
	consulClient *api.Client
}
type Config struct {
	Name                   string               `mapstructure:"name,omitempty"`
	Listen                 string               `mapstructure:"listen,omitempty"`
	Path                   string               `mapstructure:"path,omitempty"`
	Expiration             time.Duration        `mapstructure:"expiration,omitempty"`
	MetricPrefix           string               `mapstructure:"metric-prefix,omitempty"`
	AppendSubscriptionName bool                 `mapstructure:"append-subscription-name,omitempty"`
	ExportTimestamps       bool                 `mapstructure:"export-timestamps,omitempty"`
	StringsAsLabels        bool                 `mapstructure:"strings-as-labels,omitempty"`
	Debug                  bool                 `mapstructure:"debug,omitempty"`
	EventProcessors        []string             `mapstructure:"event-processors,omitempty"`
	ServiceRegistration    *ServiceRegistration `mapstructure:"service-registration,omitempty"`

	address string
	port    int
}

type ServiceRegistration struct {
	Address       string        `mapstructure:"address,omitempty"`
	Datacenter    string        `mapstructure:"datacenter,omitempty"`
	Username      string        `mapstructure:"username,omitempty"`
	Password      string        `mapstructure:"password,omitempty"`
	Token         string        `mapstructure:"token,omitempty"`
	CheckInterval time.Duration `mapstructure:"check-interval,omitempty"`
	MaxFail       int           `mapstructure:"max-fail,omitempty"`
	Name          string        `mapstructure:"name,omitempty"`
	Tags          []string      `mapstructure:"tags,omitempty"`

	deregisterAfter string
	id              string
}

func (p *PrometheusOutput) String() string {
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func (p *PrometheusOutput) SetLogger(logger *log.Logger) {
	if logger != nil && p.logger != nil {
		p.logger.SetOutput(logger.Writer())
		p.logger.SetFlags(logger.Flags())
	}
}

func (p *PrometheusOutput) SetEventProcessors(ps map[string]map[string]interface{}, log *log.Logger) {
	for _, epName := range p.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType], log)
				if err != nil {
					p.logger.Printf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
					continue
				}
				p.evps = append(p.evps, ep)
				p.logger.Printf("added event processor '%s' of type=%s to prometheus output", epName, epType)
			}
		}
	}
}

func (p *PrometheusOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
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

	err = p.setDefaults()
	if err != nil {
		return err
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
	p.registerService(wctx)
	p.logger.Printf("initialized prometheus output: %s", p.String())
	go func() {
		<-ctx.Done()
		p.Close()
	}()
	return nil
}

// Write implements the outputs.Output interface
func (p *PrometheusOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	switch rsp := rsp.(type) {
	case *gnmi.SubscribeResponse:
		measName := "default"
		if subName, ok := meta["subscription-name"]; ok {
			measName = subName
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

func (p *PrometheusOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	select {
	case <-ctx.Done():
		return
	case p.eventChan <- ev:
	}
}

func (p *PrometheusOutput) Close() error {
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

func (p *PrometheusOutput) RegisterMetrics(reg *prometheus.Registry) {}

// Describe implements prometheus.Collector
func (p *PrometheusOutput) Describe(ch chan<- *prometheus.Desc) {}

// Collect implements prometheus.Collector
func (p *PrometheusOutput) Collect(ch chan<- prometheus.Metric) {
	p.Lock()
	defer p.Unlock()
	// run expire before exporting metrics
	p.expireMetrics()
	for _, entry := range p.entries {
		ch <- entry
	}
}

func (p *PrometheusOutput) getLabels(ev *formatters.EventMsg) []*labelPair {
	labels := make([]*labelPair, 0, len(ev.Tags))
	addedLabels := make(map[string]struct{})
	for k, v := range ev.Tags {
		labelName := p.metricRegex.ReplaceAllString(filepath.Base(k), "_")
		if _, ok := addedLabels[labelName]; ok {
			continue
		}
		labels = append(labels, &labelPair{Name: labelName, Value: v})
		addedLabels[labelName] = struct{}{}
	}
	if !p.Cfg.StringsAsLabels {
		return labels
	}

	var err error
	for k, v := range ev.Values {
		_, err = getFloat(v)
		if err == nil {
			continue
		}
		if vs, ok := v.(string); ok {
			labelName := p.metricRegex.ReplaceAllString(filepath.Base(k), "_")
			if _, ok := addedLabels[labelName]; ok {
				continue
			}
			labels = append(labels, &labelPair{Name: labelName, Value: vs})
		}
	}
	return labels
}

func (p *PrometheusOutput) worker(ctx context.Context) {
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
			now := time.Now()
			labels := p.getLabels(ev)
			for vName, val := range ev.Values {
				v, err := getFloat(val)
				if err != nil {
					if !p.Cfg.StringsAsLabels {
						continue
					}
					v = 1.0
				}
				pm := &promMetric{
					name:    p.metricName(ev.Name, vName),
					labels:  labels,
					value:   v,
					addedAt: now,
				}
				if p.Cfg.ExportTimestamps {
					tm := time.Unix(0, ev.Timestamp)
					pm.time = &tm
				}
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

func (p *PrometheusOutput) expireMetrics() {
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

func (p *PrometheusOutput) expireMetricsPeriodic(ctx context.Context) {
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

func (p *PrometheusOutput) setDefaults() error {
	if p.Cfg.Listen == "" {
		p.Cfg.Listen = defaultListen
	}
	if p.Cfg.Path == "" {
		p.Cfg.Path = defaultPath
	}
	if p.Cfg.Expiration == 0 {
		p.Cfg.Expiration = defaultExpiration
	}
	if p.Cfg.ServiceRegistration != nil {
		if p.Cfg.ServiceRegistration.Address == "" {
			p.Cfg.ServiceRegistration.Address = defaultServiceRegistrationAddress
		}
		if p.Cfg.ServiceRegistration.CheckInterval <= 0 {
			p.Cfg.ServiceRegistration.CheckInterval = defaultRegistrationCheckInterval
		}
		if p.Cfg.ServiceRegistration.MaxFail <= 0 {
			p.Cfg.ServiceRegistration.MaxFail = defaultMaxServiceFail
		}
		deregisterTimer := p.Cfg.ServiceRegistration.CheckInterval * time.Duration(p.Cfg.ServiceRegistration.MaxFail)
		p.Cfg.ServiceRegistration.deregisterAfter = deregisterTimer.String()

	}
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

func (p *PrometheusOutput) registerService(ctx context.Context) {
	if p.Cfg.ServiceRegistration == nil {
		return
	}
	var err error
	clientConfig := &api.Config{
		Address:    p.Cfg.ServiceRegistration.Address,
		Scheme:     "http",
		Datacenter: p.Cfg.ServiceRegistration.Datacenter,
		Token:      p.Cfg.ServiceRegistration.Token,
	}
	if p.Cfg.ServiceRegistration.Username != "" && p.Cfg.ServiceRegistration.Password != "" {
		clientConfig.HttpAuth = &api.HttpBasicAuth{
			Username: p.Cfg.ServiceRegistration.Username,
			Password: p.Cfg.ServiceRegistration.Password,
		}
	}
INITCONSUL:
	p.consulClient, err = api.NewClient(clientConfig)
	if err != nil {
		p.logger.Printf("failed to connect to consul: %v", err)
		time.Sleep(1 * time.Second)
		goto INITCONSUL
	}

	service := &api.AgentServiceRegistration{
		ID:      p.Cfg.ServiceRegistration.id,
		Name:    p.Cfg.ServiceRegistration.Name,
		Address: p.Cfg.address,
		Port:    p.Cfg.port,
		Tags:    p.Cfg.ServiceRegistration.Tags,
		Checks: api.AgentServiceChecks{
			{
				TTL:                            p.Cfg.ServiceRegistration.CheckInterval.String(),
				DeregisterCriticalServiceAfter: p.Cfg.ServiceRegistration.deregisterAfter,
			},
			{
				HTTP:                           fmt.Sprintf("http://%s%s", p.Cfg.Listen, p.Cfg.Path),
				Method:                         "GET",
				Interval:                       p.Cfg.ServiceRegistration.CheckInterval.String(),
				TLSSkipVerify:                  true,
				DeregisterCriticalServiceAfter: p.Cfg.ServiceRegistration.deregisterAfter,
			},
		},
	}
	b, _ := json.Marshal(service)
	p.logger.Printf("registering service: %s", string(b))
	err = p.consulClient.Agent().ServiceRegister(service)
	if err != nil {
		p.logger.Printf("failed to register service in consul: %v", err)
		time.Sleep(time.Second)
		goto INITCONSUL
	}
	go func() {
		ttlCheckID := "service:" + p.Cfg.ServiceRegistration.id + ":1"
		err = p.consulClient.Agent().PassTTL(ttlCheckID, "")
		if err != nil {
			p.logger.Printf("failed to pass TTL check: %v", err)
		}
		ticker := time.NewTicker(p.Cfg.ServiceRegistration.CheckInterval / 2)
		for {
			select {
			case <-ticker.C:
				err = p.consulClient.Agent().PassTTL(ttlCheckID, "")
				if err != nil {
					p.logger.Printf("failed to pass TTL check: %v", err)
				}
			case <-ctx.Done():
				p.consulClient.Agent().FailTTL(ttlCheckID, ctx.Err().Error())
				ticker.Stop()
				return
			}
		}
	}()
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
	for _, lb := range p.labels {
		out.Label = append(out.Label, &dto.LabelPair{Name: &lb.Name, Value: &lb.Value})
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
	default:
		return math.NaN(), errors.New("getFloat: unknown value is of incompatible type")
	}
}

// metricName generates the prometheus metric name based on the output plugin,
// the measurement name and the value name.
// it makes sure the name matches the regex "[^a-zA-Z0-9_]+"
func (p *PrometheusOutput) metricName(measName, valueName string) string {
	sb := strings.Builder{}
	if p.Cfg.MetricPrefix != "" {
		sb.WriteString(p.metricRegex.ReplaceAllString(p.Cfg.MetricPrefix, "_"))
		sb.WriteString("_")
	}
	if p.Cfg.AppendSubscriptionName {
		sb.WriteString(strings.TrimRight(p.metricRegex.ReplaceAllString(measName, "_"), "_"))
		sb.WriteString("_")
	}
	sb.WriteString(strings.TrimLeft(p.metricRegex.ReplaceAllString(valueName, "_"), "_"))
	return sb.String()
}

func (p *PrometheusOutput) SetName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	if p.Cfg.Name != "" {
		sb.WriteString(p.Cfg.Name)
	}
	p.Cfg.Name = sb.String()
	if p.Cfg.ServiceRegistration != nil {
		if p.Cfg.ServiceRegistration.Name == "" {
			p.Cfg.ServiceRegistration.Name = p.Cfg.Name
		}
		if name != "" {
			p.Cfg.ServiceRegistration.id = p.Cfg.ServiceRegistration.Name + "-" + name
			return
		}
		p.Cfg.ServiceRegistration.id = p.Cfg.ServiceRegistration.Name + "-" + uuid.New().String()
	}
}
