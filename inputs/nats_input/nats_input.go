package nats_input

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/inputs"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	loggingPrefix           = "[nats_input] "
	natsReconnectBufferSize = 100 * 1024 * 1024
	defaultAddress          = "localhost:4222"
	natsConnectWait         = 2 * time.Second
	defaultFormat           = "event"
	defaultSubject          = "telemetry"
	defaultNumWorkers       = 1
	defaultBufferSize       = 100
)

func init() {
	inputs.Register("nats", func() inputs.Input {
		return &NatsInput{
			Cfg:    &Config{},
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
			wg:     new(sync.WaitGroup),
		}
	})
}

// NatsInput //
type NatsInput struct {
	Cfg    *Config
	ctx    context.Context
	cfn    context.CancelFunc
	logger *log.Logger

	wg      *sync.WaitGroup
	outputs []outputs.Output
	evps    []formatters.EventProcessor
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
	NumWorkers      int           `mapstructure:"num-workers,omitempty"`
	BufferSize      int           `mapstructure:"buffer-size,omitempty"`
	Outputs         []string      `mapstructure:"outputs,omitempty"`
	EventProcessors []string      `mapstructure:"event-processors,omitempty"`
}

// Init //
func (n *NatsInput) Start(ctx context.Context, name string, cfg map[string]interface{}, opts ...inputs.Option) error {
	err := outputs.DecodeConfig(cfg, n.Cfg)
	if err != nil {
		return err
	}
	if n.Cfg.Name == "" {
		n.Cfg.Name = name
	}
	for _, opt := range opts {
		opt(n)
	}
	err = n.setDefaults()
	if err != nil {
		return err
	}
	n.ctx, n.cfn = context.WithCancel(ctx)
	n.logger.Printf("input starting with config: %+v", n.Cfg)
	n.wg.Add(n.Cfg.NumWorkers)
	for i := 0; i < n.Cfg.NumWorkers; i++ {
		go n.worker(ctx, i)
	}
	return nil
}

func (n *NatsInput) worker(ctx context.Context, idx int) {
	var nc *nats.Conn
	var err error
	var msgChan chan *nats.Msg
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	n.logger.Printf("%s starting", workerLogPrefix)
	cfg := *n.Cfg
	cfg.Name = fmt.Sprintf("%s-%d", cfg.Name, idx)
START:
	nc, err = n.createNATSConn(&cfg)
	if err != nil {
		n.logger.Printf("%s failed to create NATS connection: %v", workerLogPrefix, err)
		time.Sleep(n.Cfg.ConnectTimeWait)
		goto START
	}
	defer nc.Close()
	msgChan = make(chan *nats.Msg, n.Cfg.BufferSize)
	sub, err := nc.ChanQueueSubscribe(n.Cfg.Subject, n.Cfg.Queue, msgChan)
	if err != nil {
		n.logger.Printf("%s failed to create NATS subscription: %v", workerLogPrefix, err)
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
				n.logger.Printf("%s channel closed, retrying...", workerLogPrefix)
				time.Sleep(n.Cfg.ConnectTimeWait)
				nc.Close()
				goto START
			}
			if len(m.Data) == 0 {
				continue
			}
			if n.Cfg.Debug {
				n.logger.Printf("received msg, subject=%s, queue=%s, len=%d, data=%s", m.Subject, m.Sub.Queue, len(m.Data), string(m.Data))
			}

			switch n.Cfg.Format {
			case "event":
				evMsgs := make([]*formatters.EventMsg, 1)
				err = json.Unmarshal(m.Data, &evMsgs)
				if err != nil {
					if n.Cfg.Debug {
						n.logger.Printf("%s failed to unmarshal event msg: %v", workerLogPrefix, err)
					}
					continue
				}

				for _, p := range n.evps {
					evMsgs = p.Apply(evMsgs...)
				}

				go func() {
					for _, o := range n.outputs {
						for _, ev := range evMsgs {
							o.WriteEvent(ctx, ev)
						}
					}
				}()
			case "proto":
				var protoMsg proto.Message
				err = proto.Unmarshal(m.Data, protoMsg)
				if err != nil {
					if n.Cfg.Debug {
						n.logger.Printf("failed to unmarshal proto msg: %v", err)
					}
					continue
				}
				meta := outputs.Meta{}
				subjectSections := strings.SplitN(m.Subject, ".", 3)
				if len(subjectSections) == 3 {
					meta["source"] = strings.ReplaceAll(subjectSections[1], "-", ".")
					meta["subscription-name"] = subjectSections[2]
				}
				go func() {
					for _, o := range n.outputs {
						o.Write(ctx, protoMsg, meta)
					}
				}()
			}

		}
	}
}

// Close //
func (n *NatsInput) Close() error {
	n.cfn()
	n.wg.Wait()
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
func (n *NatsInput) SetOutputs(outs map[string]outputs.Output) {
	if len(n.Cfg.Outputs) == 0 {
		for _, o := range outs {
			n.outputs = append(n.outputs, o)
		}
		return
	}
	for _, name := range n.Cfg.Outputs {
		if o, ok := outs[name]; ok {
			n.outputs = append(n.outputs, o)
		}
	}
}

func (n *NatsInput) SetName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(n.Cfg.Name)
	sb.WriteString("-nats-sub")
	n.Cfg.Name = sb.String()
}

func (n *NatsInput) SetEventProcessors(ps map[string]map[string]interface{}, logger *log.Logger, tcs map[string]*types.TargetConfig) {
	for _, epName := range n.Cfg.EventProcessors {
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
					n.logger.Printf("failed initializing event processor %q of type=%q: %v", epName, epType, err)
					continue
				}
				n.evps = append(n.evps, ep)
				n.logger.Printf("added event processor %q of type=%q to nats input", epName, epType)
			}
		}
	}
}

// helper functions

func (n *NatsInput) setDefaults() error {
	if n.Cfg.Format == "" {
		n.Cfg.Format = defaultFormat
	}
	if !(strings.ToLower(n.Cfg.Format) == "event" || strings.ToLower(n.Cfg.Format) == "proto") {
		return fmt.Errorf("unsupported input format")
	}
	if n.Cfg.Name == "" {
		n.Cfg.Name = "gnmic-" + uuid.New().String()
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
		n.Cfg.Queue = n.Cfg.Name
	}
	if n.Cfg.NumWorkers <= 0 {
		n.Cfg.NumWorkers = defaultNumWorkers
	}
	if n.Cfg.BufferSize <= 0 {
		n.Cfg.BufferSize = defaultBufferSize
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
