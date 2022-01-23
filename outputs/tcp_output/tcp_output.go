package tcp_output

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"text/template"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultRetryTimer = 2 * time.Second
	defaultNumWorkers = 1
	loggingPrefix     = "[tcp_output] "
)

func init() {
	outputs.Register("tcp", func() outputs.Output {
		return &TCPOutput{
			Cfg:    &Config{},
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type TCPOutput struct {
	Cfg *Config

	cancelFn context.CancelFunc
	buffer   chan []byte
	limiter  *time.Ticker
	logger   *log.Logger
	mo       *formatters.MarshalOptions
	evps     []formatters.EventProcessor

	targetTpl *template.Template
}

type Config struct {
	Address            string        `mapstructure:"address,omitempty"` // ip:port
	Rate               time.Duration `mapstructure:"rate,omitempty"`
	BufferSize         uint          `mapstructure:"buffer-size,omitempty"`
	Format             string        `mapstructure:"format,omitempty"`
	AddTarget          string        `mapstructure:"add-target,omitempty"`
	TargetTemplate     string        `mapstructure:"target-template,omitempty"`
	OverrideTimestamps bool          `mapstructure:"override-timestamps,omitempty"`
	KeepAlive          time.Duration `mapstructure:"keep-alive,omitempty"`
	RetryInterval      time.Duration `mapstructure:"retry-interval,omitempty"`
	NumWorkers         int           `mapstructure:"num-workers,omitempty"`
	EnableMetrics      bool          `mapstructure:"enable-metrics,omitempty"`
	EventProcessors    []string      `mapstructure:"event-processors,omitempty"`
}

func (t *TCPOutput) SetLogger(logger *log.Logger) {
	if logger != nil && t.logger != nil {
		t.logger.SetOutput(logger.Writer())
		t.logger.SetFlags(logger.Flags())
	}
}

func (t *TCPOutput) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) {
	for _, epName := range t.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType], formatters.WithLogger(logger), formatters.WithTargets(tcs))
				if err != nil {
					t.logger.Printf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
					continue
				}
				t.evps = append(t.evps, ep)
				t.logger.Printf("added event processor '%s' of type=%s to tcp output", epName, epType)
				continue
			}
			t.logger.Printf("%q event processor has an unknown type=%q", epName, epType)
			continue
		}
		t.logger.Printf("%q event processor not found!", epName)
	}
}

func (t *TCPOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, t.Cfg)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(t)
	}
	_, _, err = net.SplitHostPort(t.Cfg.Address)
	if err != nil {
		return fmt.Errorf("wrong address format: %v", err)
	}
	t.buffer = make(chan []byte, t.Cfg.BufferSize)
	if t.Cfg.Rate > 0 {
		t.limiter = time.NewTicker(t.Cfg.Rate)
	}
	if t.Cfg.RetryInterval == 0 {
		t.Cfg.RetryInterval = defaultRetryTimer
	}
	if t.Cfg.NumWorkers < 1 {
		t.Cfg.NumWorkers = defaultNumWorkers
	}
	t.mo = &formatters.MarshalOptions{
		Format:     t.Cfg.Format,
		OverrideTS: t.Cfg.OverrideTimestamps,
	}

	if t.Cfg.TargetTemplate == "" {
		t.targetTpl = outputs.DefaultTargetTemplate
	} else if t.Cfg.AddTarget != "" {
		t.targetTpl, err = utils.CreateTemplate("target-template", t.Cfg.TargetTemplate)
		if err != nil {
			return err
		}
		t.targetTpl = t.targetTpl.Funcs(outputs.TemplateFuncs)
	}
	go func() {
		<-ctx.Done()
		t.Close()
	}()

	ctx, t.cancelFn = context.WithCancel(ctx)
	for i := 0; i < t.Cfg.NumWorkers; i++ {
		go t.start(ctx, i)
	}
	return nil
}

func (t *TCPOutput) Write(ctx context.Context, m proto.Message, meta outputs.Meta) {
	if m == nil {
		return
	}
	var err error
	select {
	case <-ctx.Done():
		return
	default:
		err = outputs.AddSubscriptionTarget(m, meta, t.Cfg.AddTarget, t.targetTpl)
		if err != nil {
			t.logger.Printf("failed to add target to the response: %v", err)
		}
		b, err := t.mo.Marshal(m, meta, t.evps...)
		if err != nil {
			t.logger.Printf("failed marshaling proto msg: %v", err)
			return
		}
		t.buffer <- b
	}
}

func (t *TCPOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

func (t *TCPOutput) Close() error {
	t.cancelFn()
	if t.limiter != nil {
		t.limiter.Stop()
	}
	return nil
}
func (t *TCPOutput) RegisterMetrics(reg *prometheus.Registry) {}

func (t *TCPOutput) String() string {
	b, err := json.Marshal(t)
	if err != nil {
		return ""
	}
	return string(b)
}

func (t *TCPOutput) start(ctx context.Context, idx int) {
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
START:
	tcpAddr, err := net.ResolveTCPAddr("tcp", t.Cfg.Address)
	if err != nil {
		t.logger.Printf("%s failed to resolve address: %v", workerLogPrefix, err)
		time.Sleep(t.Cfg.RetryInterval)
		goto START
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		t.logger.Printf("%s failed to dial TCP: %v", workerLogPrefix, err)
		time.Sleep(t.Cfg.RetryInterval)
		goto START
	}
	defer conn.Close()
	if t.Cfg.KeepAlive > 0 {
		conn.SetKeepAlive(true)
		conn.SetKeepAlivePeriod(t.Cfg.KeepAlive)
	}
	defer t.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-t.buffer:
			if t.limiter != nil {
				<-t.limiter.C
			}
			_, err = conn.Write(b)
			if err != nil {
				t.logger.Printf("%s failed sending tcp bytes: %v", workerLogPrefix, err)
				conn.Close()
				time.Sleep(t.Cfg.RetryInterval)
				goto START
			}
		}
	}
}

func (t *TCPOutput) SetName(name string)                             {}
func (t *TCPOutput) SetClusterName(name string)                      {}
func (s *TCPOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}
