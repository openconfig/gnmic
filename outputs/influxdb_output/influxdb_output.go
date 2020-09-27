package influxdb_output

import (
	"context"
	"crypto/tls"
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
	defaultURL               = "http://localhost:8086"
	defaultBatchSize         = 1000
	defaultFlushTimer        = 10 * time.Second
	defaultHealthCheckPeriod = 30 * time.Second

	numWorkers = 1
)

func init() {
	outputs.Register("influxdb", func() outputs.Output {
		return &InfluxDBOutput{
			Cfg:       &Config{},
			eventChan: make(chan *collector.EventMsg),
		}
	})
}

type InfluxDBOutput struct {
	Cfg       *Config
	client    influxdb2.Client
	writer    api.WriteAPI
	metrics   []prometheus.Collector
	logger    *log.Logger
	cancelFn  context.CancelFunc
	eventChan chan *collector.EventMsg
}
type Config struct {
	URL               string        `mapstructure:"url,omitempty"`
	Org               string        `mapstructure:"org,omitempty"`
	Bucket            string        `mapstructure:"bucket,omitempty"`
	Token             string        `mapstructure:"token,omitempty"`
	BatchSize         uint          `mapstructure:"batch_size,omitempty"`
	FlushTimer        time.Duration `mapstructure:"flush_timer,omitempty"`
	UseGzip           bool          `mapstructure:"use_gzip,omitempty"`
	EnableTLS         bool          `mapstructure:"enable_tls,omitempty"`
	HealthCheckPeriod time.Duration `mapstructure:"health_check_period,omitempty"`
	Debug             bool          `mapstructure:"debug,omitempty"`
}

func (k *InfluxDBOutput) String() string {
	b, err := json.Marshal(k)
	if err != nil {
		return ""
	}
	return string(b)
}
func (i *InfluxDBOutput) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger) error {
	ctx, i.cancelFn = context.WithCancel(ctx)
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
	if i.Cfg.HealthCheckPeriod == 0 {
		i.Cfg.HealthCheckPeriod = defaultHealthCheckPeriod
	}
	i.logger = log.New(os.Stderr, "influxdb_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		i.logger.SetOutput(logger.Writer())
		i.logger.SetFlags(logger.Flags())
	}
	opts := influxdb2.DefaultOptions().
		SetUseGZip(i.Cfg.UseGzip).
		SetBatchSize(i.Cfg.BatchSize).
		SetFlushInterval(uint(i.Cfg.FlushTimer.Milliseconds()))
	if i.Cfg.EnableTLS {
		opts.SetTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		})
	}
	if i.Cfg.Debug {
		opts.SetLogLevel(3)
	}
	i.client = influxdb2.NewClientWithOptions(i.Cfg.URL, i.Cfg.Token, opts)
	// start influx health check
	go i.healthCheck(ctx)
	i.writer = i.client.WriteAPI(i.Cfg.Org, i.Cfg.Bucket)
	i.logger.Printf("initialized influxdb write API: %s", i.String())
	// start influxdb error logs
	go func() {
		select {
		case <-ctx.Done():
			return
		case err := <-i.writer.Errors():
			i.logger.Printf("writeAPI error: %v", err)
		}
	}()
	for k := 0; k < numWorkers; k++ {
		go i.worker(ctx, k)
	}
	go func() {
		<-ctx.Done()
		i.Close()
	}()
	return nil
}

func (i *InfluxDBOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	switch rsp := rsp.(type) {
	case *gnmi.SubscribeResponse:
		measName := "default"
		if subName, ok := meta["subscription-name"]; ok {
			measName = subName
		}
		events, err := collector.ResponseToEventMsgs(measName, rsp, meta)
		if err != nil {
			i.logger.Printf("failed to convert message to event: %v", err)
			return
		}
		for _, ev := range events {
			select {
			case <-ctx.Done():
				return
			case i.eventChan <- ev:
			}
		}
	}
}

func (i *InfluxDBOutput) Close() error {
	i.logger.Printf("flushing data...")
	i.writer.Flush()
	i.logger.Printf("closing client...")
	i.client.Close()
	i.cancelFn()
	close(i.eventChan)
	i.logger.Printf("closed.")
	return nil
}
func (i *InfluxDBOutput) Metrics() []prometheus.Collector { return i.metrics }

func (i *InfluxDBOutput) healthCheck(ctx context.Context) {
	i.health(ctx)
	ticker := time.NewTicker(i.Cfg.HealthCheckPeriod)
	for {
		select {
		case <-ticker.C:
			i.health(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (i *InfluxDBOutput) health(ctx context.Context) {
	res, err := i.client.Health(ctx)
	if err != nil {
		i.logger.Printf("failed health check: %v", err)
		return
	}
	if res != nil {
		b, err := json.Marshal(res)
		if err != nil {
			i.logger.Printf("failed to marshal health check result: %v", err)
			i.logger.Printf("health check result: %+v", res)
			return
		}
		i.logger.Printf("health check result: %s", string(b))
		return
	}
	i.logger.Print("health check result is nil")
}

func (i *InfluxDBOutput) worker(ctx context.Context, idx int) {
	select {
	case <-ctx.Done():
		i.logger.Printf("worker-%d terminating...", idx)
		return
	case ev := <-i.eventChan:
		i.writer.WritePoint(influxdb2.NewPoint(ev.Name, ev.Tags, ev.Values, time.Unix(0, ev.Timestamp)))
	}
}
