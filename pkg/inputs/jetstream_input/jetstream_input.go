// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package jetstream_input

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

	"google.golang.org/protobuf/proto"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/inputs"
	"github.com/openconfig/gnmic/pkg/outputs"
)

const (
	loggingPrefix           = "[jetstream_input:%s] "
	natsReconnectBufferSize = 100 * 1024 * 1024
	defaultAddress          = "localhost:4222"
	natsConnectWait         = 2 * time.Second
	defaultFormat           = "event"
	defaultNumWorkers       = 1
	defaultBufferSize       = 500
	defaultFetchBatchSize   = 500
)

type deliverPolicy string

const (
	deliverPolicyAll            deliverPolicy = "all"
	deliverPolicyLast           deliverPolicy = "last"
	deliverPolicyNew            deliverPolicy = "new"
	deliverPolicyLastPerSubject deliverPolicy = "last-per-subject"
)

func toJSDeliverPolicy(dp deliverPolicy) jetstream.DeliverPolicy {
	switch dp {
	case deliverPolicyAll:
		return jetstream.DeliverAllPolicy
	case deliverPolicyLast:
		return jetstream.DeliverLastPolicy
	case deliverPolicyNew:
		return jetstream.DeliverNewPolicy
	case deliverPolicyLastPerSubject:
		return jetstream.DeliverLastPerSubjectPolicy
	}
	return 0
}

func init() {
	inputs.Register("jetstream", func() inputs.Input {
		return &JetstreamInput{
			Cfg:    &Config{},
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
			wg:     new(sync.WaitGroup),
		}
	})
}

// JetstreamInput //
type JetstreamInput struct {
	Cfg    *Config
	ctx    context.Context
	cfn    context.CancelFunc
	logger *log.Logger

	wg      *sync.WaitGroup
	outputs []outputs.Output
	evps    []formatters.EventProcessor
}

type subjectFormat string

const (
	subjectFormat_Static    = "static"
	subjectFormat_TargetSub = "target.subscription"
	subjectFormat_SubTarget = "subscription.target"
)

// Config //
type Config struct {
	Name            string           `mapstructure:"name,omitempty"`
	Address         string           `mapstructure:"address,omitempty"`
	Stream          string           `mapstructure:"stream,omitempty"`
	Subjects        []string         `mapstructure:"subjects,omitempty"`
	SubjectFormat   subjectFormat    `mapstructure:"subject-format,omitempty" json:"subject-format,omitempty"`
	DeliverPolicy   deliverPolicy    `mapstructure:"deliver-policy,omitempty"`
	Username        string           `mapstructure:"username,omitempty"`
	Password        string           `mapstructure:"password,omitempty"`
	ConnectTimeWait time.Duration    `mapstructure:"connect-time-wait,omitempty"`
	TLS             *types.TLSConfig `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	Format          string           `mapstructure:"format,omitempty"`
	Debug           bool             `mapstructure:"debug,omitempty"`
	NumWorkers      int              `mapstructure:"num-workers,omitempty"`
	BufferSize      int              `mapstructure:"buffer-size,omitempty"`
	FetchBatchSize  int              `mapstructure:"fetch-batch-size,omitempty"`
	Outputs         []string         `mapstructure:"outputs,omitempty"`
	EventProcessors []string         `mapstructure:"event-processors,omitempty"`
}

// Init //
func (n *JetstreamInput) Start(ctx context.Context, name string, cfg map[string]interface{}, opts ...inputs.Option) error {
	err := outputs.DecodeConfig(cfg, n.Cfg)
	if err != nil {
		return err
	}
	if n.Cfg.Name == "" {
		n.Cfg.Name = name
	}
	n.logger.SetPrefix(fmt.Sprintf(loggingPrefix, n.Cfg.Name))
	for _, opt := range opts {
		if err := opt(n); err != nil {
			return err
		}
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

func (n *JetstreamInput) worker(ctx context.Context, idx int) {
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	n.logger.Printf("%s starting", workerLogPrefix)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := n.workerStart(ctx)
			if err != nil {
				n.logger.Printf("%s %v", workerLogPrefix, err)
				time.Sleep(n.Cfg.ConnectTimeWait)
			}
		}
	}
}

func (n *JetstreamInput) workerStart(ctx context.Context) error {
	nc, err := n.createNATSConn(n.Cfg)
	if err != nil {
		return fmt.Errorf("failed to create NATS connection: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		return fmt.Errorf("failed to create JetStream context: %v", err)
	}

	s, err := js.Stream(ctx, n.Cfg.Stream)
	if err != nil {
		return fmt.Errorf("failed to get stream: %v", err)
	}

	c, err := s.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:           n.Cfg.Name,
		Durable:        n.Cfg.Name,
		DeliverPolicy:  toJSDeliverPolicy(n.Cfg.DeliverPolicy),
		AckPolicy:      jetstream.AckAllPolicy,
		MemoryStorage:  true,
		FilterSubjects: n.Cfg.Subjects,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			mb, err := c.FetchNoWait(n.Cfg.FetchBatchSize)
			if err != nil {
				return fmt.Errorf("failed to fetch messages: %v", err)
			}
			for m := range mb.Messages() {
				n.msgHandler(m)
			}
			if mb.Error() != nil {
				return err
			}
		}
	}
}

func (n *JetstreamInput) msgHandler(msg jetstream.Msg) {
	msg.Ack()
	if n.Cfg.Debug {
		n.logger.Printf("received msg, subject=%s, len=%d, data=%s", msg.Subject(), len(msg.Data()), msg.Data())
	}

	switch n.Cfg.Format {
	case "event":
		evMsgs := make([]*formatters.EventMsg, 1)
		err := json.Unmarshal(msg.Data(), &evMsgs)
		if err != nil {
			if n.Cfg.Debug {
				n.logger.Printf("failed to unmarshal event msg: %v", err)
			}
			return
		}

		for _, p := range n.evps {
			evMsgs = p.Apply(evMsgs...)
		}

		go func() {
			for _, o := range n.outputs {
				for _, ev := range evMsgs {
					o.WriteEvent(n.ctx, ev)
				}
			}
		}()
	case "proto":
		var protoMsg = &gnmi.SubscribeResponse{}
		err := proto.Unmarshal(msg.Data(), protoMsg)
		if err != nil {
			if n.Cfg.Debug {
				n.logger.Printf("failed to unmarshal proto msg: %v", err)
			}
			return
		}

		go func() {
			for _, o := range n.outputs {
				o.Write(n.ctx, protoMsg, n.getMetaFromSubject(msg.Subject()))
			}
		}()
	default:
		n.logger.Printf("unsupported format: %s", n.Cfg.Format)
	}
}

func (n *JetstreamInput) getMetaFromSubject(subject string) outputs.Meta {
	meta := outputs.Meta{}
	subjectSections := strings.SplitN(subject, ".", 3)
	if len(subjectSections) < 3 {
		return meta
	}
	switch n.Cfg.SubjectFormat {
	case subjectFormat_Static:
	case subjectFormat_SubTarget:
		meta["subscription-name"] = subjectSections[1]
		meta["source"] = subjectSections[2]
	case subjectFormat_TargetSub:
		meta["subscription-name"] = subjectSections[2]
		meta["source"] = subjectSections[1]
	}

	return meta
}

// Close //
func (n *JetstreamInput) Close() error {
	n.cfn()
	n.wg.Wait()
	return nil
}

// SetLogger //
func (n *JetstreamInput) SetLogger(logger *log.Logger) {
	if logger != nil && n.logger != nil {
		n.logger.SetOutput(logger.Writer())
		n.logger.SetFlags(logger.Flags())
	}
}

// SetOutputs //
func (n *JetstreamInput) SetOutputs(outs map[string]outputs.Output) {
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

func (n *JetstreamInput) SetName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(n.Cfg.Name)
	sb.WriteString("-jetsream-consumer")
	n.Cfg.Name = sb.String()
}

func (n *JetstreamInput) SetEventProcessors(ps map[string]map[string]any, logger *log.Logger, tcs map[string]*types.TargetConfig, acts map[string]map[string]any) error {
	var err error
	n.evps, err = formatters.MakeEventProcessors(
		logger,
		n.Cfg.EventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return err
	}
	return nil
}

// helper functions

func (n *JetstreamInput) setDefaults() error {
	if n.Cfg.Format == "" {
		n.Cfg.Format = defaultFormat
	}
	if !(strings.ToLower(n.Cfg.Format) == "event" || strings.ToLower(n.Cfg.Format) == "proto") {
		return fmt.Errorf("unsupported input format")
	}
	if n.Cfg.Name == "" {
		n.Cfg.Name = "gnmic-jetstream-consumer" + uuid.New().String()
	}
	if n.Cfg.DeliverPolicy == "" {
		n.Cfg.DeliverPolicy = deliverPolicyAll
	}
	if n.Cfg.SubjectFormat == "" {
		n.Cfg.SubjectFormat = subjectFormat_Static
	}
	if n.Cfg.Address == "" {
		n.Cfg.Address = defaultAddress
	}
	if n.Cfg.ConnectTimeWait <= 0 {
		n.Cfg.ConnectTimeWait = natsConnectWait
	}
	if n.Cfg.NumWorkers <= 0 {
		n.Cfg.NumWorkers = defaultNumWorkers
	}
	if n.Cfg.BufferSize <= 0 {
		n.Cfg.BufferSize = defaultBufferSize
	}
	if n.Cfg.FetchBatchSize <= 0 {
		n.Cfg.FetchBatchSize = defaultFetchBatchSize
	}
	return nil
}

func (n *JetstreamInput) createNATSConn(c *Config) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(c.Name),
		nats.SetCustomDialer(n),
		nats.ReconnectWait(n.Cfg.ConnectTimeWait),
		nats.ReconnectBufSize(natsReconnectBufferSize),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			n.logger.Printf("NATS error: %v", err)
		}),
		nats.DisconnectHandler(func(*nats.Conn) {
			n.logger.Println("Disconnected from NATS")
		}),
		nats.ClosedHandler(func(*nats.Conn) {
			n.logger.Println("NATS connection is closed")
		}),
	}
	if c.Username != "" && c.Password != "" {
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	}
	if n.Cfg.TLS != nil {
		tlsConfig, err := utils.NewTLSConfig(
			n.Cfg.TLS.CaFile, n.Cfg.TLS.CertFile, n.Cfg.TLS.KeyFile, "", n.Cfg.TLS.SkipVerify,
			false)
		if err != nil {
			return nil, err
		}
		if tlsConfig != nil {
			opts = append(opts, nats.Secure(tlsConfig))
		}
	}
	nc, err := nats.Connect(c.Address, opts...)
	if err != nil {
		return nil, err
	}
	return nc, nil
}

// Dial //
func (n *JetstreamInput) Dial(network, address string) (net.Conn, error) {
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
			conn, err := d.DialContext(ctx, network, address)
			if err != nil {
				n.logger.Printf("failed to connect to NATS server %s: %v", address, err)
				time.Sleep(n.Cfg.ConnectTimeWait)
				continue
			}
			n.logger.Printf("successfully connected to NATS server %s", address)
			return conn, nil
		}
	}
}
