package stan_output

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	stanDefaultPingInterval = 5
	stanDefaultPingRetry    = 2
	defaultSubjectName      = "telemetry"
	defaultFormat           = "event"
	defaultRecoveryWaitTime = 2 * time.Second
	defaultNumWorkers       = 1
	defaultWriteTimeout     = 5 * time.Second
	defaultAddress          = "localhost:4222"
	defaultClusterName      = "test-cluster"

	loggingPrefix = "[stan_output] "
)

func init() {
	outputs.Register("stan", func() outputs.Output {
		return &StanOutput{
			Cfg:    &Config{},
			wg:     new(sync.WaitGroup),
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type protoMsg struct {
	m    proto.Message
	meta outputs.Meta
}

// StanOutput //
type StanOutput struct {
	Cfg      *Config
	cancelFn context.CancelFunc
	logger   *log.Logger
	msgChan  chan *protoMsg
	wg       *sync.WaitGroup
	mo       *formatters.MarshalOptions
	evps     []formatters.EventProcessor

	targetTpl *template.Template
}

// Config //
type Config struct {
	Name               string        `mapstructure:"name,omitempty"`
	Address            string        `mapstructure:"address,omitempty"`
	SubjectPrefix      string        `mapstructure:"subject-prefix,omitempty"`
	Subject            string        `mapstructure:"subject,omitempty"`
	Username           string        `mapstructure:"username,omitempty"`
	Password           string        `mapstructure:"password,omitempty"`
	ClusterName        string        `mapstructure:"cluster-name,omitempty"`
	PingInterval       int           `mapstructure:"ping-interval,omitempty"`
	PingRetry          int           `mapstructure:"ping-retry,omitempty"`
	Format             string        `mapstructure:"format,omitempty"`
	AddTarget          string        `mapstructure:"add-target,omitempty"`
	TargetTemplate     string        `mapstructure:"target-template,omitempty"`
	OverrideTimestamps bool          `mapstructure:"override-timestamps,omitempty"`
	RecoveryWaitTime   time.Duration `mapstructure:"recovery-wait-time,omitempty"`
	NumWorkers         int           `mapstructure:"num-workers,omitempty"`
	Debug              bool          `mapstructure:"debug,omitempty"`
	WriteTimeout       time.Duration `mapstructure:"write-timeout,omitempty"`
	EnableMetrics      bool          `mapstructure:"enable-metrics,omitempty"`
	EventProcessors    []string      `mapstructure:"event-processors,omitempty"`
}

func (s *StanOutput) String() string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *StanOutput) SetLogger(logger *log.Logger) {
	if logger != nil && s.logger != nil {
		s.logger.SetOutput(logger.Writer())
		s.logger.SetFlags(logger.Flags())
	}
}

func (s *StanOutput) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) {
	for _, epName := range s.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType], formatters.WithLogger(logger),
					formatters.WithTargets(tcs),
					formatters.WithActions(acts))
				if err != nil {
					s.logger.Printf("failed initializing event processor %q of type=%q: %v", epName, epType, err)
					continue
				}
				s.evps = append(s.evps, ep)
				s.logger.Printf("added event processor %q of type=%s to stan output", epName, epType)
				continue
			}
			s.logger.Printf("%q event processor has an unknown type=%q", epName, epType)
			continue
		}
		s.logger.Printf("%q event processor not found!", epName)
	}
}

// Init //
func (s *StanOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, s.Cfg)
	if err != nil {
		return err
	}
	if s.Cfg.Name == "" {
		s.Cfg.Name = name
	}
	for _, opt := range opts {
		opt(s)
	}
	err = s.setDefaults()
	if err != nil {
		return err
	}
	s.msgChan = make(chan *protoMsg)

	s.mo = &formatters.MarshalOptions{
		Format:     s.Cfg.Format,
		OverrideTS: s.Cfg.OverrideTimestamps,
	}

	if s.Cfg.TargetTemplate == "" {
		s.targetTpl = outputs.DefaultTargetTemplate
	} else if s.Cfg.AddTarget != "" {
		s.targetTpl, err = utils.CreateTemplate("target-template", s.Cfg.TargetTemplate)
		if err != nil {
			return err
		}
		s.targetTpl = s.targetTpl.Funcs(outputs.TemplateFuncs)
	}
	ctx, s.cancelFn = context.WithCancel(ctx)
	s.wg.Add(s.Cfg.NumWorkers)
	for i := 0; i < s.Cfg.NumWorkers; i++ {
		cfg := *s.Cfg
		cfg.Name = fmt.Sprintf("%s-%d", cfg.Name, i)
		go s.worker(ctx, i, &cfg)
	}

	s.logger.Printf("initialized stan producer: %s", s.String())
	go func() {
		<-ctx.Done()
		s.Close()
	}()
	return nil
}

func (s *StanOutput) setDefaults() error {
	if s.Cfg.Format == "" {
		s.Cfg.Format = defaultFormat
	}
	if !(s.Cfg.Format == "event" || s.Cfg.Format == "protojson" || s.Cfg.Format == "proto" || s.Cfg.Format == "json") {
		return fmt.Errorf("unsupported output format: %q for output type STAN", s.Cfg.Format)
	}
	if s.Cfg.Address == "" {
		s.Cfg.Address = defaultAddress
	}
	if s.Cfg.Name == "" {
		s.Cfg.Name = "gnmic-" + uuid.New().String()
	}
	if s.Cfg.ClusterName == "" {
		s.Cfg.ClusterName = defaultClusterName
	}
	if s.Cfg.Subject == "" && s.Cfg.SubjectPrefix == "" {
		s.Cfg.Subject = defaultSubjectName
	}
	if s.Cfg.RecoveryWaitTime == 0 {
		s.Cfg.RecoveryWaitTime = defaultRecoveryWaitTime
	}
	if s.Cfg.WriteTimeout <= 0 {
		s.Cfg.WriteTimeout = defaultWriteTimeout
	}
	if s.Cfg.NumWorkers <= 0 {
		s.Cfg.NumWorkers = defaultNumWorkers
	}
	if s.Cfg.PingInterval == 0 {
		s.Cfg.PingInterval = stanDefaultPingInterval
	}
	if s.Cfg.PingRetry == 0 {
		s.Cfg.PingRetry = stanDefaultPingRetry
	}
	return nil
}

// Write //
func (s *StanOutput) Write(ctx context.Context, rsp protoreflect.ProtoMessage, meta outputs.Meta) {
	if rsp == nil || s.mo == nil {
		return
	}

	wctx, cancel := context.WithTimeout(ctx, s.Cfg.WriteTimeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return
	case s.msgChan <- &protoMsg{m: rsp, meta: meta}:
	case <-wctx.Done():
		if s.Cfg.Debug {
			s.logger.Printf("writing expired after %s, STAN output might not be initialized", s.Cfg.WriteTimeout)
		}
		if s.Cfg.EnableMetrics {
			StanNumberOfFailSendMsgs.WithLabelValues(s.Cfg.Name, "timeout").Inc()
		}
		return
	}
}

func (s *StanOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

// Metrics //
func (s *StanOutput) RegisterMetrics(reg *prometheus.Registry) {
	if !s.Cfg.EnableMetrics {
		return
	}
	if err := registerMetrics(reg); err != nil {
		s.logger.Printf("failed to register metric: %+v", err)
	}
}

// Close //
func (s *StanOutput) Close() error {
	s.cancelFn()
	s.wg.Wait()
	return nil
}

func (s *StanOutput) createSTANConn(c *Config) stan.Conn {
	opts := []nats.Option{
		nats.Name(c.Name),
	}
	if c.Username != "" && c.Password != "" {
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	}

	var nc *nats.Conn
	var sc stan.Conn
	var err error
CRCONN:
	s.logger.Printf("attempting to connect to %s", c.Address)
	nc, err = nats.Connect(c.Address, opts...)
	if err != nil {
		s.logger.Printf("failed to create connection: %v", err)
		time.Sleep(s.Cfg.RecoveryWaitTime)
		goto CRCONN
	}
	sc, err = stan.Connect(c.ClusterName, c.Name,
		stan.NatsConn(nc),
		stan.Pings(c.PingInterval, c.PingRetry),
		stan.SetConnectionLostHandler(func(_ stan.Conn, err error) {
			s.logger.Printf("STAN connection lost, reason: %v", err)
			s.logger.Printf("retryring...")
			//sc = s.createSTANConn(c)
		}),
	)
	if err != nil {
		s.logger.Printf("failed to create connection: %v", err)
		nc.Close()
		time.Sleep(s.Cfg.RecoveryWaitTime)
		goto CRCONN
	}
	s.logger.Printf("successfully connected to STAN server %s", c.Address)
	return sc
}

func (s *StanOutput) worker(ctx context.Context, i int, c *Config) {
	defer s.wg.Done()
	var stanConn stan.Conn
	workerLogPrefix := fmt.Sprintf("worker-%d", i)
	s.logger.Printf("%s starting", workerLogPrefix)
CRCONN:
	stanConn = s.createSTANConn(c)
	s.logger.Printf("%s initialized stan producer: %s", workerLogPrefix, s.String())
	defer stanConn.Close()
	defer stanConn.NatsConn().Close()
	var err error
	for {
		select {
		case <-ctx.Done():
			s.logger.Printf("%s shutting down", workerLogPrefix)
			return
		case m := <-s.msgChan:
			err = outputs.AddSubscriptionTarget(m.m, m.meta, s.Cfg.AddTarget, s.targetTpl)
			if err != nil {
				s.logger.Printf("failed to add target to the response: %v", err)
			}
			b, err := s.mo.Marshal(m.m, m.meta, s.evps...)
			if err != nil {
				if s.Cfg.Debug {
					s.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
				}
				if s.Cfg.EnableMetrics {
					StanNumberOfFailSendMsgs.WithLabelValues(c.Name, "marshal_error").Inc()
				}
				continue
			}
			subject := s.subjectName(c, m.meta)
			start := time.Now()
			err = stanConn.Publish(subject, b)
			if err != nil {
				if s.Cfg.Debug {
					s.logger.Printf("%s failed to write to STAN subject %q: %v", workerLogPrefix, subject, err)
				}
				if s.Cfg.EnableMetrics {
					StanNumberOfFailSendMsgs.WithLabelValues(c.Name, "publish_error").Inc()
				}
				stanConn.Close()
				stanConn.NatsConn().Close()
				time.Sleep(c.RecoveryWaitTime)
				goto CRCONN
			}
			if s.Cfg.EnableMetrics {
				StanSendDuration.WithLabelValues(c.Name).Set(float64(time.Since(start).Nanoseconds()))
				StanNumberOfSentMsgs.WithLabelValues(c.Name, subject).Inc()
				StanNumberOfSentBytes.WithLabelValues(c.Name, subject).Add(float64(len(b)))
			}
		}
	}
}

func (s *StanOutput) subjectName(c *Config, meta outputs.Meta) string {
	if c.SubjectPrefix != "" {
		ssb := strings.Builder{}
		ssb.WriteString(s.Cfg.SubjectPrefix)
		if s, ok := meta["source"]; ok {
			source := strings.ReplaceAll(s, ".", "-")
			source = strings.ReplaceAll(source, " ", "_")
			ssb.WriteString(".")
			ssb.WriteString(source)
		}
		if subname, ok := meta["subscription-name"]; ok {
			ssb.WriteString(".")
			ssb.WriteString(subname)
		}
		return strings.ReplaceAll(ssb.String(), " ", "_")
	}
	return strings.ReplaceAll(s.Cfg.Subject, " ", "_")
}

func (s *StanOutput) SetName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(s.Cfg.Name)
	sb.WriteString("-stan-pub")
	s.Cfg.Name = sb.String()
}

func (s *StanOutput) SetClusterName(name string)                      {}
func (s *StanOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}
