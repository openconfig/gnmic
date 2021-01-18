package nats_input

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/inputs"
	"github.com/karimra/gnmic/outputs"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	loggingPrefix           = "nats_input "
	natsReconnectBufferSize = 100 * 1024 * 1024
	defaultAddress          = "localhost:4222"
	natsConnectWait         = 2 * time.Second
	defaultFormat           = "event"
	defaultSubject          = "telemetry"
)

func init() {
	inputs.Register("nats", func() inputs.Input {
		return &NatsInput{
			Cfg:    &Config{},
			logger: log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

// NatsInput //
type NatsInput struct {
	Cfg    *Config
	ctx    context.Context
	cfn    context.CancelFunc
	logger *log.Logger

	outputs []outputs.Output
}

// Config //
type Config struct {
	Name            string        `mapstructure:"name,omitempty"`
	Address         string        `mapstructure:"address,omitempty"`
	Subject         string        `mapstructure:"subject,omitempty"`
	Queue           string        `mapstructure:"queue,omitempty"`
	Username        string        `mapstructure:"username,omitempty"`
	Password        string        `mapstructure:"password,omitempty"`
	ConnectTimeWait time.Duration `mapstructure:"connect-time-wait,omitempty"`
	Format          string        `mapstructure:"format,omitempty"`
	Debug           bool          `mapstructure:"debug,omitempty"`
}

// Init //
func (n *NatsInput) Init(ctx context.Context, cfg map[string]interface{}, opts ...inputs.Option) error {
	err := outputs.DecodeConfig(cfg, n.Cfg)
	if err != nil {
		return err
	}
	err = n.setDefaults()
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(n)
	}
	n.ctx, n.cfn = context.WithCancel(ctx)
	return nil
}

// Start //
func (n *NatsInput) Start(ctx context.Context) {
	var nc *nats.Conn
	var err error
	var msgChan chan *nats.Msg
START:
	nc, err = n.createNATSConn(n.Cfg)
	if err != nil {
		n.logger.Printf("failed to create NATS connection: %v", err)
		time.Sleep(n.Cfg.ConnectTimeWait)
		goto START
	}
	defer nc.Close()
	msgChan = make(chan *nats.Msg)
	sub, err := nc.ChanSubscribe(n.Cfg.Subject, msgChan)
	if err != nil {
		n.logger.Printf("failed to create NATS connection: %v", err)
		time.Sleep(n.Cfg.ConnectTimeWait)
		nc.Close()
		goto START
	}
	defer close(msgChan)
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case m, ok := <-msgChan:
			if !ok {
				n.logger.Printf("channel closed, retrying...")
				time.Sleep(n.Cfg.ConnectTimeWait)
				nc.Close()
				goto START
			}
			switch n.Cfg.Format {
			case "event":
				evMsg := new(formatters.EventMsg)
				err = json.Unmarshal(m.Data, evMsg)
				if err != nil {
					n.logger.Printf("failed to unmarshal event msg")
					continue
				}
				// TODO: get meta from subject name or not? should be done already in upstream
				go func() {
					for _, o := range n.outputs {
						o.WriteEvent(ctx, evMsg)
					}
				}()
			case "proto":
				var protoMsg proto.Message
				err = proto.Unmarshal(m.Data, protoMsg)
				if err != nil {
					n.logger.Printf("failed to unmarshal proto msg")
					continue
				}
				// TODO: get meta from subject name
				go func() {
					for _, o := range n.outputs {
						o.Write(ctx, protoMsg, nil)
					}
				}()
			}
			n.logger.Printf("%s\n", string(m.Data))
		}
	}
}

// Close //
func (n *NatsInput) Close() error {
	n.cfn()
	return nil
}

// SetLogger //
func (n *NatsInput) SetLogger(logger *log.Logger) {
	if logger != nil && n.logger != nil {
		n.logger.SetOutput(logger.Writer())
		n.logger.SetFlags(logger.Flags())
	}
}

// SetOutputs //
func (n *NatsInput) SetOutputs(outs ...outputs.Output) {
	n.outputs = outs
}

// helper functions

func (n *NatsInput) setDefaults() error {
	if n.Cfg.Format == "" {
		n.Cfg.Format = defaultFormat
	}
	if !(strings.ToLower(n.Cfg.Format) == "event" || strings.ToLower(n.Cfg.Format) == "proto") {
		return fmt.Errorf("unsupported input format")
	}
	if n.Cfg.Subject == "" {
		n.Cfg.Subject = defaultSubject
	}
	if n.Cfg.Address == "" {
		n.Cfg.Address = defaultAddress
	}
	if n.Cfg.ConnectTimeWait <= 0 {
		n.Cfg.ConnectTimeWait = natsConnectWait
	}
	if n.Cfg.Queue == "" {
		n.Cfg.Queue = fmt.Sprintf("gnmic-nats-%s", uuid.New().String())
	}
	return nil
}

func (n *NatsInput) createNATSConn(c *Config) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(c.Name),
		nats.SetCustomDialer(n),
		nats.ReconnectWait(n.Cfg.ConnectTimeWait),
		nats.ReconnectBufSize(natsReconnectBufferSize),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			n.logger.Printf("NATS error: %v", err)
		}),
		nats.DisconnectHandler(func(c *nats.Conn) {
			n.logger.Println("Disconnected from NATS")
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
func (n *NatsInput) Dial(network, address string) (net.Conn, error) {
	ctx, cancel := context.WithCancel(n.ctx)
	defer cancel()

	for {
		n.logger.Printf("attempting to connect to %s", address)
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
