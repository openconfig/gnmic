package stan_output

import (
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	stanDefaultTimeout      = 10
	stanDefaultPingInterval = 5
	stanDefaultPingRetry    = 2
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
	Name         string
	Address      string
	Subject      string
	Username     string
	Password     string
	ClusterName  string
	Timeout      int
	PingInterval int
	PingRetry    int
}

// Init //
func (s *StanOutput) Init(cfg map[string]interface{}, logger *log.Logger) error {
	err := mapstructure.Decode(cfg, s.Cfg)
	if err != nil {
		return err
	}
	if s.Cfg.Name == "" {
		s.Cfg.Name = "gnmiclient-" + uuid.New().String()
	}
	if s.Cfg.ClusterName == "" {
		return fmt.Errorf("clusterName is mandatory")
	}
	s.conn, err = createSTANConn(s.Cfg)
	if err != nil {
		return err
	}
	s.logger = log.New(os.Stderr, "stan_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		s.logger.SetOutput(logger.Writer())
		s.logger.SetFlags(logger.Flags())
	}
	s.stopChan = make(chan struct{})
	s.logger.Printf("initialized stan producer")
	return nil
}

// Write //
func (s *StanOutput) Write(b []byte, meta outputs.Meta) {
	err := s.conn.Publish(s.Cfg.Subject, b)
	if err != nil {
		log.Printf("failed to write to stan subject '%s': %v", s.Cfg.Subject, err)
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

func createSTANConn(c *Config) (stan.Conn, error) {
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
		}))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %s: %v", c.Address, err)
	}
	return sc, nil
}
