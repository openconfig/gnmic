package stan_output

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	stanDefaultTimeout      = 10
	stanDefaultPingInterval = 5
	stanDefaultPingRetry    = 2

	defaultSubjectName = "gnmic-telemetry"

	defaultFormat           = "json"
	defaultRecoveryWaitTime = 10 * time.Second
)

func init() {
	outputs.Register("stan", func() outputs.Output {
		return &StanOutput{
			Cfg: &Config{},
		}
	})
}

// StanOutput //
type StanOutput struct {
	Cfg     *Config
	conn    stan.Conn
	metrics []prometheus.Collector
	logger  *log.Logger
	mo      *formatters.MarshalOptions
}

// Config //
type Config struct {
	Name             string        `mapstructure:"name,omitempty"`
	Address          string        `mapstructure:"address,omitempty"`
	SubjectPrefix    string        `mapstructure:"subject-prefix,omitempty"`
	Subject          string        `mapstructure:"subject,omitempty"`
	Username         string        `mapstructure:"username,omitempty"`
	Password         string        `mapstructure:"password,omitempty"`
	ClusterName      string        `mapstructure:"cluster-name,omitempty"`
	PingInterval     int           `mapstructure:"ping-interval,omitempty"`
	PingRetry        int           `mapstructure:"ping-retry,omitempty"`
	Format           string        `mapstructure:"format,omitempty"`
	RecoveryWaitTime time.Duration `mapstructure:"recovery-wait-time,omitempty"`
}

func (s *StanOutput) String() string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *StanOutput) SetLogger(logger *log.Logger) {
	if logger != nil {
		s.logger = log.New(logger.Writer(), "stan_output ", logger.Flags())
		return
	}
	s.logger = log.New(os.Stderr, "stan_output ", log.LstdFlags|log.Lmicroseconds)
}

// Init //
func (s *StanOutput) Init(ctx context.Context, cfg map[string]interface{}, opts ...outputs.Option) error {
	for _, opt := range opts {
		opt(s)
	}
	err := outputs.DecodeConfig(cfg, s.Cfg)
	if err != nil {
		s.logger.Printf("stan output config decode failed: %v", err)
		return err
	}
	if s.Cfg.Name == "" {
		s.Cfg.Name = "gnmic-" + uuid.New().String()
	}
	if s.Cfg.ClusterName == "" {
		s.logger.Printf("stan output config validation failed: clusterName is mandatory")
		return fmt.Errorf("clusterName is mandatory")
	}
	if s.Cfg.Subject == "" && s.Cfg.SubjectPrefix == "" {
		s.Cfg.Subject = defaultSubjectName
	}
	if s.Cfg.RecoveryWaitTime == 0 {
		s.Cfg.RecoveryWaitTime = defaultRecoveryWaitTime
	}

	if s.Cfg.Format == "" {
		s.Cfg.Format = defaultFormat
	}
	if !(s.Cfg.Format == "event" || s.Cfg.Format == "protojson" || s.Cfg.Format == "proto" || s.Cfg.Format == "json") {
		return fmt.Errorf("unsupported output format: '%s' for output type STAN", s.Cfg.Format)
	}
	if s.Cfg.PingInterval == 0 {
		s.Cfg.PingInterval = stanDefaultPingInterval
	}
	if s.Cfg.PingRetry == 0 {
		s.Cfg.PingRetry = stanDefaultPingRetry
	}
	s.mo = &formatters.MarshalOptions{Format: s.Cfg.Format}
	// this func retries until a connection is created successfully
	s.conn = s.createSTANConn(s.Cfg)
	s.logger.Printf("initialized stan producer: %s", s.String())
	go func() {
		<-ctx.Done()
		s.Close()
	}()
	return nil
}

// Write //
func (s *StanOutput) Write(_ context.Context, rsp protoreflect.ProtoMessage, meta outputs.Meta) {
	if rsp == nil || s.mo == nil {
		return
	}
	if s.conn == nil {
		return
	}

	ssb := strings.Builder{}
	ssb.WriteString(s.Cfg.SubjectPrefix)
	if s.Cfg.SubjectPrefix != "" {
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
	} else if s.Cfg.Subject != "" {
		ssb.WriteString(s.Cfg.Subject)
	}
	subject := strings.ReplaceAll(ssb.String(), " ", "_")
	b, err := s.mo.Marshal(rsp, meta)
	if err != nil {
		s.logger.Printf("failed marshaling proto msg: %v", err)
		return
	}
	err = s.conn.Publish(subject, b)
	if err != nil {
		s.logger.Printf("failed to write to stan subject '%s': %v", subject, err)
		return
	}
	//s.logger.Printf("wrote %d bytes to stan_subject=%s", len(b), s.Cfg.Subject)
}

// Metrics //
func (s *StanOutput) Metrics() []prometheus.Collector { return s.metrics }

// Close //
func (s *StanOutput) Close() error {
	return s.conn.Close()
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
			s.conn = nil
			s.logger.Printf("retryring...")
			sc = s.createSTANConn(c)
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
