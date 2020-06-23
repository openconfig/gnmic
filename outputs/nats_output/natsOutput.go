package nats_output

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/karimra/gnmiClient/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	natsConnectTimeout = 5 * time.Second
	natsConnectWait    = 2 * time.Second

	natsReconnectBufferSize = 100 * 1024 * 1024
	natsReconnectWait       = 2 * time.Second
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
	ctx      context.Context
	cancelFn context.CancelFunc
	conn     *nats.Conn
	metrics  []prometheus.Collector
	logger   *log.Logger
}

// Config //
type Config struct {
	Name            string
	Address         string
	Subject         string
	Username        string
	Password        string
	ConnectTimeout  time.Duration
	ConnectTimeWait time.Duration
}

// Init //
func (n *NatsOutput) Init(cfg map[string]interface{}, logger *log.Logger) error {
	err := mapstructure.Decode(cfg, n.Cfg)
	if err != nil {
		return err
	}
	n.Cfg.ConnectTimeout = natsConnectTimeout
	n.Cfg.ConnectTimeWait = natsConnectWait
	
	n.logger = log.New(os.Stderr, "nats_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		n.logger.SetOutput(logger.Writer())
		n.logger.SetFlags(logger.Flags())
	}
	if n.Cfg.Name == "" {
		n.Cfg.Name = "gnmiclient-" + uuid.New().String()
	}
	n.ctx, n.cancelFn = context.WithCancel(context.Background())
	n.conn, err = n.createNATSConn(n.Cfg)
	if err != nil {
		return err
	}
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
	n.cancelFn()
	n.conn.Close()
	return nil
}

// Metrics //
func (n *NatsOutput) Metrics() []prometheus.Collector { return n.metrics }

func (n *NatsOutput) createNATSConn(c *Config) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(c.Name),
		nats.SetCustomDialer(n),
		nats.ReconnectWait(natsReconnectWait),
		nats.ReconnectBufSize(natsReconnectBufferSize),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			n.logger.Printf("nats error: %v", err)
		}),
		nats.DisconnectHandler(func(c *nats.Conn) {
			n.logger.Println("disconnected from NATS")
		}),
		nats.ClosedHandler(func(c *nats.Conn) {
			n.logger.Println("NATS connection is closed")
		}),
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

// Dial //
func (n *NatsOutput) Dial(network, address string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(n.ctx, n.Cfg.ConnectTimeout)
	defer cancel()

	for {
		n.logger.Println("attempting to connect to", address)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		select {
		case <-n.ctx.Done():
			return nil, n.ctx.Err()
		default:
			d := &net.Dialer{}
			if conn, err := d.DialContext(ctx, network, address); err == nil {
				n.logger.Printf("successfully connected to NATS server %s", address)
				return conn, nil
			}
			time.Sleep(n.Cfg.ConnectTimeWait)
		}
	}
}
