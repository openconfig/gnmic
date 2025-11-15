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
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	"github.com/openconfig/gnmic/pkg/store"
	gutils "github.com/openconfig/gnmic/pkg/utils"
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
		return &NatsOutput{}
	})
}

func (n *NatsOutput) init() {
	n.cfg = new(atomic.Pointer[Config])
	n.dynCfg = new(atomic.Pointer[dynConfig])
	n.msgChan = new(atomic.Pointer[chan *outputs.ProtoMsg])
	n.wg = new(sync.WaitGroup)
	n.logger = log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags)
}

// NatsOutput //
type NatsOutput struct {
	outputs.BaseOutput
	// Cfg *Config
	cfg    *atomic.Pointer[Config]
	dynCfg *atomic.Pointer[dynConfig]
	// root context
	ctx      context.Context
	cancelFn context.CancelFunc
	msgChan  *atomic.Pointer[chan *outputs.ProtoMsg] // atomic channel swaps
	wg       *sync.WaitGroup
	logger   *log.Logger

	reg   *prometheus.Registry
	store store.Store[any]
}

type dynConfig struct {
	targetTpl *template.Template
	msgTpl    *template.Template
	evps      []formatters.EventProcessor
	mo        *formatters.MarshalOptions
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
	cfg := n.cfg.Load()
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (n *NatsOutput) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(n.store)
	if err != nil {
		return nil, err
	}
	evps, err := formatters.MakeEventProcessors(
		logger,
		eventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return nil, err
	}
	return evps, nil
}

func (n *NatsOutput) setLogger(logger *log.Logger) {
	if logger != nil && n.logger != nil {
		n.logger.SetOutput(logger.Writer())
		n.logger.SetFlags(logger.Flags())
	}
}

// Init //
func (n *NatsOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	n.init() // init struct fields
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	if newCfg.Name == "" {
		newCfg.Name = name
	}
	n.logger.SetPrefix(fmt.Sprintf(loggingPrefix, newCfg.Name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}
	n.store = options.Store
	// set defaults
	n.setDefaultsFor(newCfg)

	n.cfg.Store(newCfg)

	// apply logger
	n.setLogger(options.Logger)

	// initialize registry
	n.reg = options.Registry
	err = n.registerMetrics()
	if err != nil {
		return err
	}

	// initialize message channel
	msgChan := make(chan *outputs.ProtoMsg, newCfg.BufferSize)
	n.msgChan.Store(&msgChan)

	// prep dynamic config
	dc := new(dynConfig)

	dc.mo = &formatters.MarshalOptions{
		Format:     newCfg.Format,
		OverrideTS: newCfg.OverrideTimestamps,
	}
	// initialize event processors
	dc.evps, err = n.buildEventProcessors(options.Logger, newCfg.EventProcessors)
	if err != nil {
		return err
	}

	// initialize target template
	if newCfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	// initialize message template
	if newCfg.MsgTemplate != "" {
		dc.msgTpl, err = gtemplate.CreateTemplate("msg-template", newCfg.MsgTemplate)
		if err != nil {
			return err
		}
		dc.msgTpl = dc.msgTpl.Funcs(outputs.TemplateFuncs)
	}

	n.dynCfg = new(atomic.Pointer[dynConfig])
	n.dynCfg.Store(dc)

	// initialize context
	n.ctx, n.cancelFn = context.WithCancel(ctx)
	n.wg.Add(newCfg.NumWorkers)
	for i := 0; i < newCfg.NumWorkers; i++ {
		go n.worker(n.ctx, i)
	}

	return nil
}

func (n *NatsOutput) setDefaultsFor(cfg *Config) {
	if cfg.Format == "" {
		cfg.Format = defaultFormat
	}
	if cfg.Address == "" {
		cfg.Address = defaultAddress
	}
	if cfg.ConnectTimeWait <= 0 {
		cfg.ConnectTimeWait = natsConnectWait
	}
	if cfg.Subject == "" && cfg.SubjectPrefix == "" {
		cfg.Subject = defaultSubjectName
	}
	if cfg.Name == "" {
		cfg.Name = "gnmic-" + uuid.New().String()
	}
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = defaultNumWorkers
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}
}

func (n *NatsOutput) Validate(cfg map[string]any) error {
	ncfg := new(Config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}
	_, err = gtemplate.CreateTemplate("target-template", ncfg.TargetTemplate)
	if err != nil {
		return err
	}
	_, err = gtemplate.CreateTemplate("msg-template", ncfg.MsgTemplate)
	if err != nil {
		return err
	}
	return nil
}

func (n *NatsOutput) Update(ctx context.Context, cfg map[string]any) error {
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	n.setDefaultsFor(newCfg)
	currCfg := n.cfg.Load()

	swapChannel := channelNeedsSwap(currCfg, newCfg)
	restartWorkers := needsWorkerRestart(currCfg, newCfg)
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0

	var targetTpl *template.Template
	if newCfg.TargetTemplate == "" {
		targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		t, err := gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		targetTpl = t.Funcs(outputs.TemplateFuncs)
	} else {
		targetTpl = outputs.DefaultTargetTemplate
	}

	var msgTpl *template.Template
	if newCfg.MsgTemplate != "" {
		t, err := gtemplate.CreateTemplate("msg-template", newCfg.MsgTemplate)
		if err != nil {
			return err
		}
		msgTpl = t.Funcs(outputs.TemplateFuncs)
	}

	dc := &dynConfig{
		targetTpl: targetTpl,
		msgTpl:    msgTpl,
		mo: &formatters.MarshalOptions{
			Format:     newCfg.Format,
			OverrideTS: newCfg.OverrideTimestamps,
		},
	}

	prevDC := n.dynCfg.Load()
	if rebuildProcessors {
		dc.evps, err = n.buildEventProcessors(n.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}
	n.dynCfg.Store(dc)
	n.cfg.Store(newCfg)

	if swapChannel || restartWorkers {
		var newMsgChan chan *outputs.ProtoMsg
		if swapChannel {
			newMsgChan = make(chan *outputs.ProtoMsg, newCfg.BufferSize)
		} else {
			newMsgChan = *n.msgChan.Load()
		}

		runCtx, cancel := context.WithCancel(n.ctx)
		newWG := new(sync.WaitGroup)
		// save old pointers
		oldCancel := n.cancelFn
		oldWG := n.wg
		oldMsgChan := *n.msgChan.Load()
		// swap
		n.cancelFn = cancel
		n.wg = newWG
		n.msgChan.Store(&newMsgChan)

		n.wg.Add(currCfg.NumWorkers)
		for i := 0; i < currCfg.NumWorkers; i++ {
			go n.worker(runCtx, i)
		}
		// cancel old workers and loops
		if oldCancel != nil {
			oldCancel()
		}
		if oldWG != nil {
			oldWG.Wait()
		}
		if swapChannel {
			// best effort drain old channel
		OUTER_LOOP:
			for {
				select {
				case msg, ok := <-oldMsgChan:
					if !ok {
						break
					}
					select {
					case newMsgChan <- msg:
					default:
					}
				default:
					break OUTER_LOOP
				}
			}
		}
	}
	n.logger.Printf("updated nats output: %s", n.String())
	return nil

}

func (n *NatsOutput) UpdateProcessor(name string, pcfg map[string]any) error {
	cfg := n.cfg.Load()
	dc := n.dynCfg.Load()

	newEvps, changed, err := outputs.UpdateProcessorInSlice(
		n.logger,
		n.store,
		cfg.EventProcessors,
		dc.evps,
		name,
		pcfg,
	)
	if err != nil {
		return err
	}
	if changed {
		newDC := *dc
		newDC.evps = newEvps
		n.dynCfg.Store(&newDC)
		n.logger.Printf("updated event processor %s", name)
	}
	return nil
}

// Write //
func (n *NatsOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	dc := n.dynCfg.Load()
	cfg := n.cfg.Load()
	if rsp == nil || dc == nil || dc.mo == nil {
		return
	}

	wctx, cancel := context.WithTimeout(ctx, cfg.WriteTimeout)
	defer cancel()

	ch := n.msgChan.Load()
	select {
	case <-ctx.Done():
		return
	case *ch <- outputs.NewProtoMsg(rsp, meta):
	case <-wctx.Done():
		if cfg.Debug {
			n.logger.Printf("writing expired after %s, NATS output might not be initialized", cfg.WriteTimeout)
		}
		if cfg.EnableMetrics {
			NatsNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "timeout").Inc()
		}
		return
	}
}

func (n *NatsOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

// Close //
func (n *NatsOutput) Close() error {
	n.cancelFn()
	n.wg.Wait()
	n.logger.Printf("closed nats output: %s", n.String())
	return nil
}

func (n *NatsOutput) createNATSConn(c *Config, i int) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(fmt.Sprintf("%s-%d", c.Name, i)),
		nats.SetCustomDialer(n),
		nats.ReconnectWait(c.ConnectTimeWait),
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
	if c.TLS != nil {
		tlsConfig, err := utils.NewTLSConfig(
			c.TLS.CaFile,
			c.TLS.CertFile,
			c.TLS.KeyFile,
			"",
			c.TLS.SkipVerify,
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
		cfg := n.cfg.Load()
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
				time.Sleep(cfg.ConnectTimeWait)
				continue
			}
			n.logger.Printf("successfully connected to NATS server %s", address)
			return conn, nil
		}
	}
}

func (n *NatsOutput) worker(ctx context.Context, i int) {
	defer n.wg.Done()
	var natsConn *nats.Conn
	var err error
	workerLogPrefix := fmt.Sprintf("worker-%d", i)

	defer n.logger.Printf("%s exited", workerLogPrefix)
	n.logger.Printf("%s starting", workerLogPrefix)
	msgChan := *n.msgChan.Load()
CRCONN:
	if ctx.Err() != nil {
		return
	}
	cfg := n.cfg.Load()
	natsConn, err = n.createNATSConn(cfg, i)
	if err != nil {
		n.logger.Printf("%s failed to create connection: %v", workerLogPrefix, err)
		time.Sleep(cfg.ConnectTimeWait)
		goto CRCONN
	}
	for {
		select {
		case <-ctx.Done():
			n.logger.Printf("%s flushing", workerLogPrefix)
			natsConn.FlushTimeout(time.Second)
			n.logger.Printf("%s shutting down", workerLogPrefix)
			natsConn.Close()
			return
		case m := <-msgChan:
			pmsg := m.GetMsg()
			// get fresh config
			cfg := n.cfg.Load()
			// snapshot template and marshal options
			dc := n.dynCfg.Load()
			name := fmt.Sprintf("%s-%d", cfg.Name, i)
			pmsg, err = outputs.AddSubscriptionTarget(pmsg, m.GetMeta(), cfg.AddTarget, dc.targetTpl)
			if err != nil {
				n.logger.Printf("failed to add target to the response: %v", err)
			}
			bb, err := outputs.Marshal(pmsg, m.GetMeta(), dc.mo, cfg.SplitEvents, dc.evps...)
			if err != nil {
				if cfg.Debug {
					n.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
				}
				if cfg.EnableMetrics {
					NatsNumberOfFailSendMsgs.WithLabelValues(name, "marshal_error").Inc()
				}
				continue
			}
			if len(bb) == 0 {
				continue
			}
			for _, b := range bb {
				if dc.msgTpl != nil {
					b, err = outputs.ExecTemplate(b, dc.msgTpl)
					if err != nil {
						if cfg.Debug {
							log.Printf("failed to execute template: %v", err)
						}
						NatsNumberOfFailSendMsgs.WithLabelValues(name, "template_error").Inc()
						continue
					}
				}

				subject := n.subjectName(m.GetMeta(), cfg)
				var start time.Time
				if cfg.EnableMetrics {
					start = time.Now()
				}
				err = natsConn.Publish(subject, b)
				if err != nil {
					if cfg.Debug {
						n.logger.Printf("%s failed to write to nats subject '%s': %v", workerLogPrefix, subject, err)
					}
					if cfg.EnableMetrics {
						NatsNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "publish_error").Inc()
					}
					if cfg.Debug {
						n.logger.Printf("%s closing connection to NATS '%s'", workerLogPrefix, subject)
					}

					natsConn.Close()
					time.Sleep(cfg.ConnectTimeWait)

					if cfg.Debug {
						n.logger.Printf("%s reconnecting to NATS", workerLogPrefix)
					}
					goto CRCONN
				}
				if cfg.EnableMetrics {
					NatsSendDuration.WithLabelValues(name).Set(float64(time.Since(start).Nanoseconds()))
					NatsNumberOfSentMsgs.WithLabelValues(name, subject).Inc()
					NatsNumberOfSentBytes.WithLabelValues(name, subject).Add(float64(len(b)))
				}
			}
		}
	}
}

var stringBuilderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

func (n *NatsOutput) subjectName(meta outputs.Meta, cfg *Config) string {
	if cfg.SubjectPrefix != "" {
		ssb := stringBuilderPool.Get().(*strings.Builder)
		defer func() {
			ssb.Reset()
			stringBuilderPool.Put(ssb)
		}()
		ssb.WriteString(cfg.SubjectPrefix)

		if s, ok := meta["source"]; ok {
			ssb.WriteString(".")
			for _, r := range s {
				switch r {
				case '.':
					ssb.WriteRune('-')
				case ' ':
					ssb.WriteRune('_')
				default:
					ssb.WriteRune(r)
				}
			}
		}

		if subname, ok := meta["subscription-name"]; ok {
			ssb.WriteString(".")
			for _, r := range subname {
				if r == ' ' {
					ssb.WriteRune('_')
				} else {
					ssb.WriteRune(r)
				}
			}
		}

		return ssb.String()
	}
	return strings.ReplaceAll(cfg.Subject, " ", "_")
}

func channelNeedsSwap(old, nw *Config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.BufferSize != nw.BufferSize
}

func needsWorkerRestart(old, nw *Config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.NumWorkers != nw.NumWorkers ||
		!old.TLS.Equal(nw.TLS) ||
		old.Address != nw.Address ||
		old.Username != nw.Username ||
		old.Password != nw.Password
}
