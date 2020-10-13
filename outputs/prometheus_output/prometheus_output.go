package prometheus_output

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultListen     = ":9273"
	defaultPath       = "/metrics"
	defaultExpiration = time.Minute
)

type labelPair struct {
	Name  string
	Value string
}
type promMetric struct {
	name   string
	labels []labelPair
	time   time.Time
	value  float64
}

func init() {
	outputs.Register("prometheus", func() outputs.Output {
		return &PrometheusOutput{
			Cfg:       &Config{},
			eventChan: make(chan *collector.EventMsg),
		}
	})
}

type PrometheusOutput struct {
	Cfg       *Config
	metrics   []prometheus.Collector
	logger    *log.Logger
	cancelFn  context.CancelFunc
	eventChan chan *collector.EventMsg

	sync.Mutex
	entries map[uint64]*promMetric
}
type Config struct {
	Listen        string
	Path          string
	Expiration    time.Duration
	StringAslabel bool
}

func (p *PrometheusOutput) String() string {
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}
func (p *PrometheusOutput) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger) error {
	ctx, p.cancelFn = context.WithCancel(ctx)
	err := mapstructure.Decode(cfg, p.Cfg)
	if err != nil {
		return err
	}
	if p.Cfg.Listen == "" {
		p.Cfg.Listen = defaultListen
	}
	if p.Cfg.Path == "" {
		p.Cfg.Path = defaultPath
	}
	if p.Cfg.Expiration == 0 {
		p.Cfg.Expiration = defaultExpiration
	}
	p.logger = log.New(os.Stderr, "prometheus_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		p.logger.SetOutput(logger.Writer())
		p.logger.SetFlags(logger.Flags())
	}
	return nil
}
func (p *PrometheusOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {}
func (p *PrometheusOutput) Close() error                                                    { return nil }
func (p *PrometheusOutput) Metrics() []prometheus.Collector                                 { return p.metrics }

///
func (p *PrometheusOutput) Describe(ch chan<- *prometheus.Desc) {}
func (p *PrometheusOutput) Collect(ch chan<- prometheus.Metric) {
}

func (p *PrometheusOutput) getLabels(ev *collector.EventMsg) []labelPair {
	labels := make([]labelPair, 0, len(ev.Tags))
	for k, v := range ev.Tags {
		labels = append(labels, labelPair{Name: filepath.Base(k), Value: v})
	}
	if p.Cfg.StringAslabel {
		for k, v := range ev.Values {
			switch v := v.(type) {
			case string:
				labels = append(labels, labelPair{Name: filepath.Base(k), Value: v})
			}
		}
	}
	return labels
}

func (p *promMetric) calculateKey() uint64 {
	h := fnv.New64a()
	h.Write([]byte(p.name))
	for _, label := range p.labels {
		h.Write([]byte(label.Name))
		h.Write([]byte(":"))
		h.Write([]byte(label.Value))
		h.Write([]byte(":"))
	}
	return h.Sum64()
}
