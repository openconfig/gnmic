package influxdb_output

import (
	"encoding/json"
	"log"
	"time"

	"github.com/Shopify/sarama"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultURL        = "http://localhost:8086"
	defaultBatchSize  = 1000
	defaultFlushTimer = 10 * time.Second
)

func init() {
	outputs.Register("influxdb", func() outputs.Output {
		return &InfluxDBOutput{
			Cfg: &Config{},
		}
	})
}

type InfluxDBOutput struct {
	Cfg     *Config
	client  influxdb2.Client
	metrics []prometheus.Collector
	logger  sarama.StdLogger
	mo      *collector.MarshalOptions
}
type Config struct {
	URL        string        `mapstructure:"url,omitempty"`
	Username   string        `mapstructure:"username,omitempty"`
	Password   string        `mapstructure:"password,omitempty"`
	Token      string        `mapstructure:"token,omitempty"`
	BatchSize  uint64        `mapstructure:"batch_size,omitempty"`
	FlushTimer time.Duration `mapstructure:"flush_timer,omitempty"`
}

func (k *InfluxDBOutput) String() string {
	b, err := json.Marshal(k)
	if err != nil {
		return ""
	}
	return string(b)
}
func (i *InfluxDBOutput) Init(cfg map[string]interface{}, logger *log.Logger) error {
	err := mapstructure.Decode(cfg, i.Cfg)
	if err != nil {
		return err
	}
	if i.Cfg.URL == "" {
		i.Cfg.URL = defaultURL
	}
	if i.Cfg.BatchSize == 0 {
		i.Cfg.BatchSize = defaultBatchSize
	}
	if i.Cfg.FlushTimer == 0 {
		i.Cfg.FlushTimer = defaultFlushTimer
	}
	i.client = influxdb2.NewClient(i.Cfg.URL, "")

	return nil
}

func (i *InfluxDBOutput) Write(rsp proto.Message, meta outputs.Meta) {}
func (i *InfluxDBOutput) Close() error                               { return nil }
func (i *InfluxDBOutput) Metrics() []prometheus.Collector            { return i.metrics }
