package influxdb_output

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"math"
	"strings"
	"text/template"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultURL               = "http://localhost:8086"
	defaultBatchSize         = 1000
	defaultFlushTimer        = 10 * time.Second
	defaultHealthCheckPeriod = 30 * time.Second

	numWorkers    = 1
	loggingPrefix = "[influxdb_output] "
)

func init() {
	outputs.Register("influxdb", func() outputs.Output {
		return &InfluxDBOutput{
			Cfg:       &Config{},
			eventChan: make(chan *formatters.EventMsg),
			reset:     make(chan struct{}),
			startSig:  make(chan struct{}),
			logger:    log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type InfluxDBOutput struct {
	Cfg       *Config
	client    influxdb2.Client
	logger    *log.Logger
	cancelFn  context.CancelFunc
	eventChan chan *formatters.EventMsg
	reset     chan struct{}
	startSig  chan struct{}
	wasUP     bool
	evps      []formatters.EventProcessor
	dbVersion string

	targetTpl *template.Template
}
type Config struct {
	URL                string        `mapstructure:"url,omitempty"`
	Org                string        `mapstructure:"org,omitempty"`
	Bucket             string        `mapstructure:"bucket,omitempty"`
	Token              string        `mapstructure:"token,omitempty"`
	BatchSize          uint          `mapstructure:"batch-size,omitempty"`
	FlushTimer         time.Duration `mapstructure:"flush-timer,omitempty"`
	UseGzip            bool          `mapstructure:"use-gzip,omitempty"`
	EnableTLS          bool          `mapstructure:"enable-tls,omitempty"`
	HealthCheckPeriod  time.Duration `mapstructure:"health-check-period,omitempty"`
	Debug              bool          `mapstructure:"debug,omitempty"`
	AddTarget          string        `mapstructure:"add-target,omitempty"`
	TargetTemplate     string        `mapstructure:"target-template,omitempty"`
	EventProcessors    []string      `mapstructure:"event-processors,omitempty"`
	EnableMetrics      bool          `mapstructure:"enable-metrics,omitempty"`
	OverrideTimestamps bool          `mapstructure:"override-timestamps,omitempty"`
}

func (k *InfluxDBOutput) String() string {
	b, err := json.Marshal(k)
	if err != nil {
		return ""
	}
	return string(b)
}

func (i *InfluxDBOutput) SetLogger(logger *log.Logger) {
	if logger != nil && i.logger != nil {
		i.logger.SetOutput(logger.Writer())
		i.logger.SetFlags(logger.Flags())
	}
}

func (i *InfluxDBOutput) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) {
	for _, epName := range i.Cfg.EventProcessors {
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
					i.logger.Printf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
					continue
				}
				i.evps = append(i.evps, ep)
				i.logger.Printf("added event processor '%s' of type=%s to influxdb output", epName, epType)
				continue
			}
			i.logger.Printf("%q event processor has an unknown type=%q", epName, epType)
			continue
		}
		i.logger.Printf("%q event processor not found!", epName)
	}
}

func (i *InfluxDBOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, i.Cfg)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(i)
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

	iopts := influxdb2.DefaultOptions().
		SetUseGZip(i.Cfg.UseGzip).
		SetBatchSize(i.Cfg.BatchSize).
		SetFlushInterval(uint(i.Cfg.FlushTimer.Milliseconds()))
	if i.Cfg.EnableTLS {
		iopts.SetTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		})
	}
	if i.Cfg.TargetTemplate == "" {
		i.targetTpl = outputs.DefaultTargetTemplate
	} else if i.Cfg.AddTarget != "" {
		i.targetTpl, err = utils.CreateTemplate("target-template", i.Cfg.TargetTemplate)
		if err != nil {
			return err
		}
		i.targetTpl = i.targetTpl.Funcs(outputs.TemplateFuncs)
	}
	if i.Cfg.Debug {
		iopts.SetLogLevel(3)
	}
	ctx, i.cancelFn = context.WithCancel(ctx)
CRCLIENT:
	i.client = influxdb2.NewClientWithOptions(i.Cfg.URL, i.Cfg.Token, iopts)
	// start influx health check
	err = i.health(ctx)
	if err != nil {
		i.logger.Printf("failed to check influxdb health: %v", err)
		time.Sleep(10 * time.Second)
		goto CRCLIENT
	}
	i.wasUP = true
	go i.healthCheck(ctx)
	i.logger.Printf("initialized influxdb client: %s", i.String())

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
	err := outputs.AddSubscriptionTarget(rsp, meta, i.Cfg.AddTarget, i.targetTpl)
	if err != nil {
		i.logger.Printf("failed to add target to the response: %v", err)
	}
	switch rsp := rsp.(type) {
	case *gnmi.SubscribeResponse:
		measName := "default"
		if subName, ok := meta["subscription-name"]; ok {
			measName = subName
		}
		events, err := formatters.ResponseToEventMsgs(measName, rsp, meta, i.evps...)
		if err != nil {
			i.logger.Printf("failed to convert message to event: %v", err)
			return
		}
		for _, ev := range events {
			select {
			case <-ctx.Done():
				return
			case <-i.reset:
				return
			case i.eventChan <- ev:
			}
		}
	}
}

func (i *InfluxDBOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	select {
	case <-ctx.Done():
		return
	case <-i.reset:
		return
	case i.eventChan <- ev:
	}
}

func (i *InfluxDBOutput) Close() error {
	i.logger.Printf("closing client...")
	i.cancelFn()
	i.logger.Printf("closed.")
	return nil
}
func (i *InfluxDBOutput) RegisterMetrics(reg *prometheus.Registry) {}

func (i *InfluxDBOutput) healthCheck(ctx context.Context) {
	ticker := time.NewTicker(i.Cfg.HealthCheckPeriod)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			i.health(ctx)
		}
	}
}

func (i *InfluxDBOutput) health(ctx context.Context) error {
	res, err := i.client.Health(ctx)
	if err != nil {
		i.logger.Printf("failed health check: %v", err)
		if i.wasUP {
			close(i.reset)
			i.reset = make(chan struct{})
		}
		return err
	}
	if res != nil {
		if res.Version != nil {
			i.dbVersion = *res.Version
		}
		b, err := json.Marshal(res)
		if err != nil {
			i.logger.Printf("failed to marshal health check result: %v", err)
			i.logger.Printf("health check result: %+v", res)
			if i.wasUP {
				close(i.reset)
				i.reset = make(chan struct{})
			}
			return err
		}
		i.wasUP = true
		close(i.startSig)
		i.startSig = make(chan struct{})
		i.logger.Printf("health check result: %s", string(b))
		return nil
	}
	i.wasUP = true
	close(i.startSig)
	i.startSig = make(chan struct{})
	i.logger.Print("health check result is nil")
	return nil
}

func (i *InfluxDBOutput) worker(ctx context.Context, idx int) {
	firstStart := true
START:
	if !firstStart {
		i.logger.Printf("worker-%d waiting for client recovery", idx)
		<-i.startSig
	}
	i.logger.Printf("starting worker-%d", idx)
	writer := i.client.WriteAPI(i.Cfg.Org, i.Cfg.Bucket)
	//defer writer.Flush()
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				i.logger.Printf("worker-%d err=%v", idx, ctx.Err())
			}
			i.logger.Printf("worker-%d terminating...", idx)
			return
		case ev := <-i.eventChan:
			if len(ev.Values) == 0 {
				continue
			}
			for n, v := range ev.Values {
				switch v := v.(type) {
				case *gnmi.Decimal64:
					ev.Values[n] = float64(v.Digits) / math.Pow10(int(v.Precision))
				}
			}
			if ev.Timestamp == 0 || i.Cfg.OverrideTimestamps {
				ev.Timestamp = time.Now().UnixNano()
			}
			i.convertUints(ev)
			writer.WritePoint(influxdb2.NewPoint(ev.Name, ev.Tags, ev.Values, time.Unix(0, ev.Timestamp)))
		case <-i.reset:
			firstStart = false
			i.logger.Printf("resetting worker-%d...", idx)
			goto START
		case err := <-writer.Errors():
			i.logger.Printf("worker-%d write error: %v", idx, err)
		}
	}
}

func (i *InfluxDBOutput) SetName(name string)                             {}
func (i *InfluxDBOutput) SetClusterName(name string)                      {}
func (i *InfluxDBOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}

func (i *InfluxDBOutput) convertUints(ev *formatters.EventMsg) {
	if !strings.HasPrefix(i.dbVersion, "1.8") {
		return
	}
	for k, v := range ev.Values {
		switch v := v.(type) {
		case uint:
			ev.Values[k] = int(v)
		case uint8:
			ev.Values[k] = int(v)
		case uint16:
			ev.Values[k] = int(v)
		case uint32:
			ev.Values[k] = int(v)
		case uint64:
			ev.Values[k] = int(v)
		}
	}
}
