package stan_output

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
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
	Cfg      *Config
	conn     stan.Conn
	metrics  []prometheus.Collector
	logger   *log.Logger
	stopChan chan struct{}
}

// Config //
type Config struct {
	Name          string `mapstructure:"name,omitempty"`
	Address       string `mapstructure:"address,omitempty"`
	SubjectPrefix string `mapstructure:"subject-prefix,omitempty"`
	Subject       string `mapstructure:"subject,omitempty"`
	Username      string `mapstructure:"username,omitempty"`
	Password      string `mapstructure:"password,omitempty"`
	ClusterName   string `mapstructure:"cluster-name,omitempty"`
	Timeout       int    `mapstructure:"timeout,omitempty"`
	PingInterval  int    `mapstructure:"ping-interval,omitempty"`
	PingRetry     int    `mapstructure:"ping-retry,omitempty"`
	Format        string `mapstructure:"format,omitempty"`
}

func (s *StanOutput) String() string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(b)
}

// Init //
func (s *StanOutput) Init(cfg map[string]interface{}, logger *log.Logger) error {
	err := mapstructure.Decode(cfg, s.Cfg)
	if err != nil {
		return err
	}
	if s.Cfg.Name == "" {
		s.Cfg.Name = "gnmic-" + uuid.New().String()
	}
	if s.Cfg.ClusterName == "" {
		return fmt.Errorf("clusterName is mandatory")
	}
	if s.Cfg.Subject == "" && s.Cfg.SubjectPrefix == "" {
		s.Cfg.Subject = defaultSubjectName
	}
	s.conn, err = s.createSTANConn(s.Cfg)
	if err != nil {
		return err
	}
	s.logger = log.New(os.Stderr, "stan_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		s.logger.SetOutput(logger.Writer())
		s.logger.SetFlags(logger.Flags())
	}
	if s.Cfg.Format == "" {
		s.Cfg.Format = "json"
	}
	if !(s.Cfg.Format == "event" || s.Cfg.Format == "protojson" || s.Cfg.Format == "proto" || s.Cfg.Format == "json") {
		return fmt.Errorf("unsupported output format: '%s' for output type STAN", s.Cfg.Format)
	}
	s.stopChan = make(chan struct{})
	s.logger.Printf("initialized stan producer: %s", s.String())
	return nil
}

// Write //
func (s *StanOutput) Write(rsp protoreflect.ProtoMessage, meta outputs.Meta) {
	if rsp == nil {
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
	b, err := collector.Marshal(rsp, s.Cfg.Format, meta, false, "")
	if err != nil {
		s.logger.Printf("failed marshaling proto msg: %v", err)
		return
	}
	err = s.conn.Publish(subject, b)
	if err != nil {
		log.Printf("failed to write to stan subject '%s': %v", subject, err)
		return
	}
	//s.logger.Printf("wrote %d bytes to stan_subject=%s", len(b), s.Cfg.Subject)
}

// Metrics //
func (s *StanOutput) Metrics() []prometheus.Collector { return s.metrics }

// Close //
func (s *StanOutput) Close() error {
	close(s.stopChan)
	return s.conn.Close()
}

func (s *StanOutput) createSTANConn(c *Config) (stan.Conn, error) {
	if c.Timeout == 0 {
		c.Timeout = stanDefaultTimeout
	}
	if c.PingInterval == 0 {
		c.PingInterval = stanDefaultPingInterval
	}
	if c.PingRetry == 0 {
		c.PingRetry = stanDefaultPingRetry
	}
	opts := []nats.Option{
		nats.Name(c.Name),
		// nats.Timeout(time.Duration(c.Timeout) * time.Second),
		// nats.PingInterval(time.Duration(c.PingInterval) * time.Second),
		// nats.MaxPingsOutstanding(c.PingRetry),
	}
	if c.Username != "" && c.Password != "" {
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	}
	nc, err := nats.Connect(c.Address, opts...)
	if err != nil {
		return nil, err
	}
	sc, err := stan.Connect(c.ClusterName, c.Name,
		stan.NatsConn(nc),
		stan.Pings(c.PingInterval, c.PingRetry),
		stan.SetConnectionLostHandler(func(_ stan.Conn, err error) {
			log.Fatalf("STAN connection lost, reason: %v", err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %s: %v", c.Address, err)
	}
	return sc, nil
}
