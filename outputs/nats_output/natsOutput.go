package nats_output

import (
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/karimra/gnmiClient/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/nats-io/nats.go"
)

const (
	natsDefaultTimeout      = 10 * time.Second
	natsDefaultPingInterval = 5 * time.Second
	natsDefaultPingRetry    = 2
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

// Initialize //
func (n *NatsOutput) Initialize(cfg map[string]interface{}) error {
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
	n.stopChan = make(chan struct{})
	return nil
}

// Write //
func (n *NatsOutput) Write(b []byte) {
	err := n.conn.Publish(n.Cfg.Subject, b)
	if err != nil {
		log.Printf("failed to write to nats subject '%s': %v", n.Cfg.Subject, err)
		return
	}
}

// Close //
func (n *NatsOutput) Close() error {
	close(n.stopChan)
	n.conn.Close()
	return nil
}

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
