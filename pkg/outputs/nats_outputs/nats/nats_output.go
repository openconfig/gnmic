// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package nats_output

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
)

const (
	natsConnectWait         = 2 * time.Second
	natsReconnectBufferSize = 100 * 1024 * 1024
	defaultSubjectName      = "telemetry"
	defaultFormat           = "event"
	defaultNumWorkers       = 1
	defaultWriteTimeout     = 5 * time.Second
	defaultAddress          = "localhost:4222"
	loggingPrefix           = "[nats_output:%s] "
)

func init() {
	outputs.Register("nats", func() outputs.Output {
		return &NatsOutput{
			Cfg:    &Config{},
			wg:     new(sync.WaitGroup),
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// NatsOutput //
type NatsOutput struct {
	outputs.BaseOutput
	Cfg      *Config
	ctx      context.Context
	cancelFn context.CancelFunc
	msgChan  chan *outputs.ProtoMsg
	wg       *sync.WaitGroup
	logger   *log.Logger
	mo       *formatters.MarshalOptions
	evps     []formatters.EventProcessor

	targetTpl *template.Template
	msgTpl    *template.Template

	reg *prometheus.Registry
}

// Config //
type Config struct {
	Name               string           `mapstructure:"name,omitempty"`
	Address            string           `mapstructure:"address,omitempty"`
	SubjectPrefix      string           `mapstructure:"subject-prefix,omitempty"`
	Subject            string           `mapstructure:"subject,omitempty"`
	Username           string           `mapstructure:"username,omitempty"`
	Password           string           `mapstructure:"password,omitempty"`
	ConnectTimeWait    time.Duration    `mapstructure:"connect-time-wait,omitempty"`
	TLS                *types.TLSConfig `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	Format             string           `mapstructure:"format,omitempty"`
	SplitEvents        bool             `mapstructure:"split-events,omitempty"`
	AddTarget          string           `mapstructure:"add-target,omitempty"`
	TargetTemplate     string           `mapstructure:"target-template,omitempty"`
	MsgTemplate        string           `mapstructure:"msg-template,omitempty"`
	OverrideTimestamps bool             `mapstructure:"override-timestamps,omitempty"`
	NumWorkers         int              `mapstructure:"num-workers,omitempty"`
	WriteTimeout       time.Duration    `mapstructure:"write-timeout,omitempty"`
	Debug              bool             `mapstructure:"debug,omitempty"`
	BufferSize         uint             `mapstructure:"buffer-size,omitempty"`
	EnableMetrics      bool             `mapstructure:"enable-metrics,omitempty"`
	EventProcessors    []string         `mapstructure:"event-processors,omitempty"`
}

func (n *NatsOutput) String() string {
	b, err := json.Marshal(n)
	if err != nil {
		return ""
	}
	return string(b)
}

// Init //
func (n *NatsOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, n.Cfg)
	if err != nil {
		return err
	}
	if n.Cfg.Name == "" {
		n.Cfg.Name = name
	}
	n.logger.SetPrefix(fmt.Sprintf(loggingPrefix, n.Cfg.Name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	// set defaults
	n.setName(options.Name)
	err = n.setDefaults()
	if err != nil {
		return err
	}

	// apply logger
	if options.Logger != nil && n.logger != nil {
		n.logger.SetOutput(options.Logger.Writer())
		n.logger.SetFlags(options.Logger.Flags())
	}

	// initialize registry
	n.reg = options.Registry
	err = n.registerMetrics()
	if err != nil {
		return err
	}

	// initialize event processors
	n.evps, err = formatters.MakeEventProcessors(options.Logger, n.Cfg.EventProcessors,
		options.EventProcessors, options.TargetsConfig, options.Actions)
	if err != nil {
		return err
	}

	// initialize message channel
	n.msgChan = make(chan *outputs.ProtoMsg, n.Cfg.BufferSize)
	n.mo = &formatters.MarshalOptions{
		Format:     n.Cfg.Format,
		OverrideTS: n.Cfg.OverrideTimestamps,
	}

	// initialize target template
	if n.Cfg.TargetTemplate == "" {
		n.targetTpl = outputs.DefaultTargetTemplate
	} else if n.Cfg.AddTarget != "" {
		n.targetTpl, err = gtemplate.CreateTemplate("target-template", n.Cfg.TargetTemplate)
		if err != nil {
			return err
		}
		n.targetTpl = n.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	// initialize message template
	if n.Cfg.MsgTemplate != "" {
		n.msgTpl, err = gtemplate.CreateTemplate("msg-template", n.Cfg.MsgTemplate)
		if err != nil {
			return err
		}
		n.msgTpl = n.msgTpl.Funcs(outputs.TemplateFuncs)
	}

	// initialize context
	n.ctx, n.cancelFn = context.WithCancel(ctx)
	n.wg.Add(n.Cfg.NumWorkers)
	for i := 0; i < n.Cfg.NumWorkers; i++ {
		cfg := *n.Cfg
		cfg.Name = fmt.Sprintf("%s-%d", cfg.Name, i)
		go n.worker(ctx, i, &cfg)
	}

	go func() {
		<-ctx.Done()
		n.Close()
	}()
	return nil
}

func (n *NatsOutput) setDefaults() error {
	if n.Cfg.Format == "" {
		n.Cfg.Format = defaultFormat
	}
	if !(n.Cfg.Format == "event" || n.Cfg.Format == "protojson" || n.Cfg.Format == "proto" || n.Cfg.Format == "json") {
		return fmt.Errorf("unsupported output format '%s' for output type NATS", n.Cfg.Format)
	}
	if n.Cfg.Address == "" {
		n.Cfg.Address = defaultAddress
	}
	if n.Cfg.ConnectTimeWait <= 0 {
		n.Cfg.ConnectTimeWait = natsConnectWait
	}
	if n.Cfg.Subject == "" && n.Cfg.SubjectPrefix == "" {
		n.Cfg.Subject = defaultSubjectName
	}
	if n.Cfg.Name == "" {
		n.Cfg.Name = "gnmic-" + uuid.New().String()
	}
	if n.Cfg.NumWorkers <= 0 {
		n.Cfg.NumWorkers = defaultNumWorkers
	}
	if n.Cfg.WriteTimeout <= 0 {
		n.Cfg.WriteTimeout = defaultWriteTimeout
	}
	return nil
}

// Write //
func (n *NatsOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil || n.mo == nil {
		return
	}

	wctx, cancel := context.WithTimeout(ctx, n.Cfg.WriteTimeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return
	case n.msgChan <- outputs.NewProtoMsg(rsp, meta):
	case <-wctx.Done():
		if n.Cfg.Debug {
			n.logger.Printf("writing expired after %s, NATS output might not be initialized", n.Cfg.WriteTimeout)
		}
		if n.Cfg.EnableMetrics {
			NatsNumberOfFailSendMsgs.WithLabelValues(n.Cfg.Name, "timeout").Inc()
		}
		return
	}
}

func (n *NatsOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

// Close //
func (n *NatsOutput) Close() error {
	//	n.conn.Close()
	n.cancelFn()
	n.wg.Wait()
	return nil
}

func (n *NatsOutput) createNATSConn(c *Config) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(c.Name),
		nats.SetCustomDialer(n),
		nats.ReconnectWait(n.Cfg.ConnectTimeWait),
		nats.ReconnectBufSize(natsReconnectBufferSize),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			n.logger.Printf("NATS error: %v", err)
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			n.logger.Printf("Disconnected from NATS err=%v", err)
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
			n.Cfg.TLS.CaFile,
			n.Cfg.TLS.CertFile,
			n.Cfg.TLS.KeyFile,
			"",
			n.Cfg.TLS.SkipVerify,
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
func (n *NatsOutput) Dial(network, address string) (net.Conn, error) {
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

func (n *NatsOutput) worker(ctx context.Context, i int, cfg *Config) {
	defer n.wg.Done()

	var natsConn *nats.Conn
	var err error
	workerLogPrefix := fmt.Sprintf("worker-%d", i)

	defer n.logger.Printf("%s exited", workerLogPrefix)
	n.logger.Printf("%s starting", workerLogPrefix)
CRCONN:
	natsConn, err = n.createNATSConn(cfg)
	if err != nil {
		n.logger.Printf("%s failed to create connection: %v", workerLogPrefix, err)
		time.Sleep(n.Cfg.ConnectTimeWait)
		goto CRCONN
	}
	defer n.logger.Printf("%s initialized nats publisher: %+v", workerLogPrefix, cfg)
	for {
		select {
		case <-ctx.Done():
			n.logger.Printf("%s flushing", workerLogPrefix)
			natsConn.FlushTimeout(time.Second)
			n.logger.Printf("%s shutting down", workerLogPrefix)
			natsConn.Close()
			return
		case m := <-n.msgChan:
			pmsg := m.GetMsg()
			pmsg, err = outputs.AddSubscriptionTarget(pmsg, m.GetMeta(), n.Cfg.AddTarget, n.targetTpl)
			if err != nil {
				n.logger.Printf("failed to add target to the response: %v", err)
			}
			bb, err := outputs.Marshal(pmsg, m.GetMeta(), n.mo, n.Cfg.SplitEvents, n.evps...)
			if err != nil {
				if n.Cfg.Debug {
					n.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
				}
				if n.Cfg.EnableMetrics {
					NatsNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "marshal_error").Inc()
				}
				continue
			}
			if len(bb) == 0 {
				continue
			}
			for _, b := range bb {
				if n.msgTpl != nil {
					b, err = outputs.ExecTemplate(b, n.msgTpl)
					if err != nil {
						if n.Cfg.Debug {
							log.Printf("failed to execute template: %v", err)
						}
						NatsNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "template_error").Inc()
						continue
					}
				}

				subject := n.subjectName(cfg, m.GetMeta())
				var start time.Time
				if n.Cfg.EnableMetrics {
					start = time.Now()
				}
				err = natsConn.Publish(subject, b)
				if err != nil {
					if n.Cfg.Debug {
						n.logger.Printf("%s failed to write to nats subject '%s': %v", workerLogPrefix, subject, err)
					}
					if n.Cfg.EnableMetrics {
						NatsNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "publish_error").Inc()
					}
					if n.Cfg.Debug {
						n.logger.Printf("%s closing connection to NATS '%s'", workerLogPrefix, subject)
					}

					natsConn.Close()
					time.Sleep(cfg.ConnectTimeWait)

					if n.Cfg.Debug {
						n.logger.Printf("%s reconnecting to NATS", workerLogPrefix)
					}
					goto CRCONN
				}
				if n.Cfg.EnableMetrics {
					NatsSendDuration.WithLabelValues(cfg.Name).Set(float64(time.Since(start).Nanoseconds()))
					NatsNumberOfSentMsgs.WithLabelValues(cfg.Name, subject).Inc()
					NatsNumberOfSentBytes.WithLabelValues(cfg.Name, subject).Add(float64(len(b)))
				}
			}
		}
	}
}

func (n *NatsOutput) subjectName(c *Config, meta outputs.Meta) string {
	if c.SubjectPrefix != "" {
		ssb := strings.Builder{}
		ssb.WriteString(n.Cfg.SubjectPrefix)
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
	return strings.ReplaceAll(n.Cfg.Subject, " ", "_")
}

func (n *NatsOutput) setName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(n.Cfg.Name)
	sb.WriteString("-nats-pub")
	n.Cfg.Name = sb.String()
}
