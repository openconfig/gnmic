package influxdb_output

import (
	"encoding/json"
	"log"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
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
	writer  api.WriteAPI
	metrics []prometheus.Collector
	logger  *log.Logger
}
type Config struct {
	URL        string        `mapstructure:"url,omitempty"`
	Org        string        `mapstructure:"org,omitempty"`
	Bucket     string        `mapstructure:"bucket,omitempty"`
	Token      string        `mapstructure:"token,omitempty"`
	BatchSize  uint          `mapstructure:"batch_size,omitempty"`
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
	i.logger = log.New(os.Stderr, "influxdb_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		i.logger.SetOutput(logger.Writer())
		i.logger.SetFlags(logger.Flags())
	}
	i.client = influxdb2.NewClientWithOptions(i.Cfg.URL, i.Cfg.Token,
		influxdb2.DefaultOptions().
			SetBatchSize(i.Cfg.BatchSize).
			SetFlushInterval(uint(i.Cfg.FlushTimer.Milliseconds())))
	i.writer = i.client.WriteAPI(i.Cfg.Org, i.Cfg.Bucket)
	i.logger.Printf("initialized influxdb write API: %s", i.String())
	go func() {
		for err = range i.writer.Errors() {
			i.logger.Printf("writeAPI error: %v", err)
		}
	}()
	return nil
}

func (i *InfluxDBOutput) Write(rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	switch rsp := rsp.(type) {
	case *gnmi.SubscribeResponse:
		events, err := collector.ResponseToEventMsgs(i.Cfg.Bucket, rsp, meta)
		if err != nil {
			i.logger.Printf("failed to convert message to event: %v", err)
			return
		}
		for _, ev := range events {
			i.writer.WritePoint(influxdb2.NewPoint(ev.Name, ev.Tags, ev.Values, time.Unix(0, ev.Timestamp)))
		}
	}
}

func (i *InfluxDBOutput) Close() error {
	i.logger.Printf("flushing data...")
	i.writer.Flush()
	i.logger.Printf("closing client...")
	i.client.Close()
	i.logger.Printf("closed.")
	return nil
}
func (i *InfluxDBOutput) Metrics() []prometheus.Collector { return i.metrics }
