package nats_output

import (
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/karimra/gnmiClient/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	outputs.Register("nats", func() outputs.Output {
		return &NatsOutput{
			Cfg: &Config{},
		}
	})
}

// NatsOutput //
type NatsOutput struct {
	Cfg      *Config
	conn     *nats.Conn
	metrics  []prometheus.Collector
	logger   *log.Logger
	stopChan chan struct{}
}

// Config //
type Config struct {
	Name     string
	Address  string
	Subject  string
	Username string
	Password string
}

// Init //
func (n *NatsOutput) Init(cfg map[string]interface{}, logger *log.Logger) error {
	err := mapstructure.Decode(cfg, n.Cfg)
	if err != nil {
		return err
	}
	if n.Cfg.Name == "" {
		n.Cfg.Name = "gnmiclient-" + uuid.New().String()
	}
	n.conn, err = createNATSConn(n.Cfg)
	if err != nil {
		return err
	}
	n.logger = log.New(os.Stderr, "nats_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		n.logger.SetOutput(logger.Writer())
		n.logger.SetFlags(logger.Flags())
	}
	n.stopChan = make(chan struct{})
	n.logger.Printf("initialized nats producer")
	return nil
}

// Write //
func (n *NatsOutput) Write(b []byte) {
	err := n.conn.Publish(n.Cfg.Subject, b)
	if err != nil {
		log.Printf("failed to write to nats subject '%s': %v", n.Cfg.Subject, err)
		return
	}
	// n.logger.Printf("wrote %d bytes to nats_subject=%s", len(b), n.Cfg.Subject)
}

// Close //
func (n *NatsOutput) Close() error {
	close(n.stopChan)
	n.conn.Close()
	return nil
}

// Metrics //
func (n *NatsOutput) Metrics() []prometheus.Collector { return n.metrics }

func createNATSConn(c *Config) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(c.Name),
	}
	if c.Username != "" && c.Password != "" {
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	}
	nc, err := nats.Connect(c.Address, opts...)
	if err != nil {
		return nil, err
	}
	return nc, nil
}
