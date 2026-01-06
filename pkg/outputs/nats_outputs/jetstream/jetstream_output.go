// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package jetstream_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	gutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
)

const (
	loggingPrefix       = "[jetstream_output:%s] "
	defaultSubjectName  = "telemetry"
	defaultFormat       = "event"
	defaultAddress      = "localhost:4222"
	natsConnectWait     = 2 * time.Second
	defaultNumWorkers   = 1
	defaultWriteTimeout = 5 * time.Second
)

func init() {
	outputs.Register("jetstream", func() outputs.Output {
		return &jetstreamOutput{}
	})
}

type subjectFormat string

const (
	subjectFormat_Static                = "static"
	subjectFormat_TargetSub             = "target.subscription"
	subjectFormat_SubTarget             = "subscription.target"
	subjectFormat_SubTargetPath         = "subscription.target.path"
	subjectFormat_SubTargetPathWithKeys = "subscription.target.pathKeys"
)

type config struct {
	Name               string              `mapstructure:"name,omitempty" json:"name,omitempty"`
	Address            string              `mapstructure:"address,omitempty" json:"address,omitempty"`
	Stream             string              `mapstructure:"stream,omitempty" json:"stream,omitempty"`
	Subject            string              `mapstructure:"subject,omitempty" json:"subject,omitempty"`
	SubjectFormat      subjectFormat       `mapstructure:"subject-format,omitempty" json:"subject-format,omitempty"`
	CreateStream       *createStreamConfig `mapstructure:"create-stream,omitempty" json:"create-stream,omitempty"`
	Username           string              `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password           string              `mapstructure:"password,omitempty" json:"password,omitempty"`
	ConnectTimeWait    time.Duration       `mapstructure:"connect-time-wait,omitempty" json:"connect-time-wait,omitempty"`
	TLS                *types.TLSConfig    `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	Format             string              `mapstructure:"format,omitempty" json:"format,omitempty"`
	SplitEvents        bool                `mapstructure:"split-events,omitempty" json:"split-events,omitempty"`
	AddTarget          string              `mapstructure:"add-target,omitempty" json:"add-target,omitempty"`
	TargetTemplate     string              `mapstructure:"target-template,omitempty" json:"target-template,omitempty"`
	MsgTemplate        string              `mapstructure:"msg-template,omitempty" json:"msg-template,omitempty"`
	OverrideTimestamps bool                `mapstructure:"override-timestamps,omitempty" json:"override-timestamps,omitempty"`
	NumWorkers         int                 `mapstructure:"num-workers,omitempty" json:"num-workers,omitempty"`
	WriteTimeout       time.Duration       `mapstructure:"write-timeout,omitempty" json:"write-timeout,omitempty"`
	Debug              bool                `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	BufferSize         uint                `mapstructure:"buffer-size,omitempty" json:"buffer-size,omitempty"`
	EnableMetrics      bool                `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	EventProcessors    []string            `mapstructure:"event-processors,omitempty" json:"event-processors,omitempty"`
}

type createStreamConfig struct {
	Description string        `mapstructure:"description,omitempty" json:"description,omitempty"`
	Subjects    []string      `mapstructure:"subjects,omitempty" json:"subjects,omitempty"`
	Storage     string        `mapstructure:"storage,omitempty" json:"storage,omitempty"`
	Retention   string        `mapstructure:"retention-policy,omitempty" json:"retention-policy,omitempty"`
	MaxMsgs     int64         `mapstructure:"max-msgs,omitempty" json:"max-msgs,omitempty"`
	MaxBytes    int64         `mapstructure:"max-bytes,omitempty" json:"max-bytes,omitempty"`
	MaxAge      time.Duration `mapstructure:"max-age,omitempty" json:"max-age,omitempty"`
	MaxMsgSize  int32         `mapstructure:"max-msg-size,omitempty" json:"max-msg-size,omitempty"`
}

// jetstreamOutput //
type jetstreamOutput struct {
	outputs.BaseOutput

	cfg      *atomic.Pointer[config]
	rootCtx  context.Context
	cancelFn context.CancelFunc

	msgChan *atomic.Pointer[chan *outputs.ProtoMsg] // atomic channel swaps
	// workers wait group
	wg *sync.WaitGroup
	// dynamic config items that don't need a worker restart
	dynCfg *atomic.Pointer[dynConfig]
	// metrics registry
	reg *prometheus.Registry
	// config store
	store  store.Store[any]
	logger *log.Logger

	closeOnce sync.Once
	closeSig  chan struct{}
}

type dynConfig struct {
	targetTpl *template.Template
	msgTpl    *template.Template
	evps      []formatters.EventProcessor
	mo        *formatters.MarshalOptions
}

func (n *jetstreamOutput) init() {
	n.cfg = new(atomic.Pointer[config])
	n.dynCfg = new(atomic.Pointer[dynConfig])
	n.msgChan = new(atomic.Pointer[chan *outputs.ProtoMsg])
	n.wg = new(sync.WaitGroup)
	n.logger = log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags)
	n.closeOnce = sync.Once{}
	n.closeSig = make(chan struct{})
}

func (n *jetstreamOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	n.init() // init struct fields
	ncfg := new(config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}
	if ncfg.Name == "" {
		ncfg.Name = name
	}
	n.logger.SetPrefix(fmt.Sprintf(loggingPrefix, ncfg.Name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	n.store = options.Store

	// set defaults
	err = n.setDefaultsFor(ncfg)
	if err != nil {
		return err
	}

	n.cfg.Store(ncfg)
	// apply logger
	n.setLogger(options.Logger)

	// initialize registry
	n.reg = options.Registry
	err = n.registerMetrics()
	if err != nil {
		return err
	}

	msgChan := make(chan *outputs.ProtoMsg, ncfg.BufferSize)
	n.msgChan.Store(&msgChan)
	// prep dynamic config
	dc := new(dynConfig)
	// initialize event processors
	evps, err := n.buildEventProcessors(options.Logger, ncfg.EventProcessors)
	if err != nil {
		return err
	}
	dc.evps = evps

	dc.mo = &formatters.MarshalOptions{
		Format:     ncfg.Format,
		OverrideTS: ncfg.OverrideTimestamps,
	}
	if ncfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if ncfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", ncfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	if ncfg.MsgTemplate != "" {
		dc.msgTpl, err = gtemplate.CreateTemplate("msg-template", ncfg.MsgTemplate)
		if err != nil {
			return err
		}
		dc.msgTpl = dc.msgTpl.Funcs(outputs.TemplateFuncs)
	}

	n.dynCfg.Store(dc)

	n.rootCtx = ctx // store root context
	var wctx context.Context

	wctx, n.cancelFn = context.WithCancel(n.rootCtx) // create worker context
	n.wg.Add(ncfg.NumWorkers)
	for i := 0; i < ncfg.NumWorkers; i++ {
		go n.worker(wctx, i)
	}

	return nil
}

func (n *jetstreamOutput) setDefaultsFor(cfg *config) error {
	if cfg.Stream == "" {
		return errors.New("missing stream name")
	}
	if cfg.Format == "" {
		cfg.Format = defaultFormat
	}
	if cfg.SubjectFormat == "" {
		cfg.SubjectFormat = subjectFormat_Static
	}
	switch cfg.SubjectFormat {
	case subjectFormat_Static,
		subjectFormat_TargetSub,
		subjectFormat_SubTarget,
		subjectFormat_SubTargetPath,
		subjectFormat_SubTargetPathWithKeys:
	default:
		return fmt.Errorf("unknown subject-format value: %v", cfg.SubjectFormat)
	}
	if cfg.Subject == "" {
		cfg.Subject = defaultSubjectName
	}
	if cfg.Address == "" {
		cfg.Address = defaultAddress
	}
	if cfg.ConnectTimeWait <= 0 {
		cfg.ConnectTimeWait = natsConnectWait
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
	if cfg.CreateStream != nil {
		if len(cfg.CreateStream.Subjects) == 0 {
			cfg.CreateStream.Subjects = []string{fmt.Sprintf("%s.>", cfg.Stream)}
		}
		if cfg.CreateStream.Description == "" {
			cfg.CreateStream.Description = "created by gNMIc"
		}
		if cfg.CreateStream.Storage == "" {
			cfg.CreateStream.Storage = "memory"
		}
		if cfg.CreateStream.Retention == "" {
			cfg.CreateStream.Retention = "limits"
		}
		// Validate retention policy value
		if !isValidRetentionPolicy(cfg.CreateStream.Retention) {
			return fmt.Errorf("invalid retention-policy: %s (must be 'limits' or 'workqueue')",
				cfg.CreateStream.Retention)
		}
		return nil
	}
	return nil
}

func (n *jetstreamOutput) Validate(cfg map[string]any) error {
	ncfg := new(config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}
	err = n.setDefaultsFor(ncfg)
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

func (n *jetstreamOutput) Update(ctx context.Context, cfg map[string]any) error {
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	err = n.setDefaultsFor(newCfg)
	if err != nil {
		return err
	}
	currCfg := n.cfg.Load()

	swapChannel := channelNeedsSwap(currCfg, newCfg)
	restartWorkers := needsWorkerRestart(currCfg, newCfg)
	streamChanged := streamChanged(currCfg, newCfg)
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0

	//rebuild
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

	// rebuild processors ?
	prevDC := n.dynCfg.Load()
	if rebuildProcessors {
		dc.evps, err = n.buildEventProcessors(n.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}
	// store new dynamic config
	n.dynCfg.Store(dc)
	// store new config
	n.cfg.Store(newCfg)

	if swapChannel || restartWorkers || streamChanged {
		var newChan chan *outputs.ProtoMsg
		if swapChannel {
			newChan = make(chan *outputs.ProtoMsg, newCfg.BufferSize)

		} else {
			newChan = *n.msgChan.Load()
		}

		runCtx, cancel := context.WithCancel(n.rootCtx)
		newWG := new(sync.WaitGroup)
		// save old pointers
		oldCancel := n.cancelFn
		oldWG := n.wg
		oldMsgChan := *n.msgChan.Load()
		// swap
		n.cancelFn = cancel
		n.wg = newWG
		n.msgChan.Store(&newChan)

		// restart workers
		n.wg.Add(currCfg.NumWorkers)
		for i := 0; i < currCfg.NumWorkers; i++ {
			go n.worker(runCtx, i)
		}
		// cancel old workers
		if oldCancel != nil {
			oldCancel()
		}
		if oldWG != nil {
			oldWG.Wait()
		}
		if swapChannel {
			// best effort drain old channel
		OUTER_LOOP: // break label
			for {
				select {
				case msg, ok := <-oldMsgChan:
					if !ok {
						break
					}
					select {
					case newChan <- msg:
					default:
						// new channel full, drop message
					}
				default:
					break OUTER_LOOP // break out of the outer loop
				}
			}
		}
	}
	n.logger.Printf("updated jetstream output: %s", n.String())
	return nil

}

func (n *jetstreamOutput) UpdateProcessor(name string, pcfg map[string]any) error {
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

func (n *jetstreamOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
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
	case <-n.closeSig:
		return
	case <-wctx.Done():
		if cfg.Debug {
			n.logger.Printf("writing expired after %s, JetStream output might not be initialized", cfg.WriteTimeout)
		}
		if cfg.EnableMetrics {
			jetStreamNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "timeout").Inc()
		}
		return
	}
}

func (n *jetstreamOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

func (n *jetstreamOutput) Close() error {
	n.cancelFn()
	n.wg.Wait()
	n.closeOnce.Do(func() {
		close(n.closeSig)
	})
	n.logger.Printf("closed jetstream output: %s", n.String())
	return nil
}

func (c *config) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}

func (n *jetstreamOutput) String() string {
	cfg := n.cfg.Load()
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (n *jetstreamOutput) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
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

func (n *jetstreamOutput) setLogger(logger *log.Logger) {
	if logger != nil && n.logger != nil {
		n.logger.SetOutput(logger.Writer())
		n.logger.SetFlags(logger.Flags())
	}
}

func (n *jetstreamOutput) worker(ctx context.Context, i int) {
	defer n.wg.Done()
	var natsConn *nats.Conn
	var err error
	var subject string
	workerLogPrefix := fmt.Sprintf("worker-%d", i)
	n.logger.Printf("%s starting", workerLogPrefix)
	// snapshot msgChan
	msgChan := *n.msgChan.Load()
CRCONN:
	if ctx.Err() != nil {
		return
	}
	cfg := n.cfg.Load()
	name := fmt.Sprintf("%s-%d", cfg.Name, i)
	natsConn, err = n.createNATSConn(ctx, cfg, i)
	if err != nil {
		n.logger.Printf("%s failed to create connection: %v", workerLogPrefix, err)
		time.Sleep(cfg.ConnectTimeWait)
		goto CRCONN
	}
	js, err := natsConn.JetStream()
	if err != nil {
		if cfg.Debug {
			n.logger.Printf("%s failed to create jetstream context: %v", workerLogPrefix, err)
		}
		if cfg.EnableMetrics {
			jetStreamNumberOfFailSendMsgs.WithLabelValues(name, "jetstream_context_error").Inc()
		}
		natsConn.Close()
		time.Sleep(cfg.ConnectTimeWait)
		goto CRCONN
	}
	n.logger.Printf("%s initialized nats jetstream producer: %s", workerLogPrefix, cfg)
	// worker-0 create stream if configured
	if i == 0 {
		err = n.createStream(js, cfg)
		if err != nil {
			if cfg.Debug {
				n.logger.Printf("%s failed to create stream: %v", workerLogPrefix, err)
			}
			if cfg.EnableMetrics {
				jetStreamNumberOfFailSendMsgs.WithLabelValues(name, "create_stream_error").Inc()
			}
			natsConn.Close()
			time.Sleep(cfg.ConnectTimeWait)
			goto CRCONN
		}
	}

	for {
		select {
		case <-ctx.Done():
			natsConn.Close()
			n.logger.Printf("%s shutting down", workerLogPrefix)
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
			var rs []proto.Message
			switch cfg.SubjectFormat {
			case subjectFormat_Static, subjectFormat_TargetSub, subjectFormat_SubTarget:
				rs = []proto.Message{pmsg}
			case subjectFormat_SubTargetPath, subjectFormat_SubTargetPathWithKeys:
				switch rsp := pmsg.(type) {
				case *gnmi.SubscribeResponse:
					switch rsp := rsp.Response.(type) {
					case *gnmi.SubscribeResponse_Update:
						rs = splitSubscribeResponse(rsp)
					}
				}
			}
			for _, r := range rs {
				bb, err := outputs.Marshal(r, m.GetMeta(), dc.mo, cfg.SplitEvents, dc.evps...)
				if err != nil {
					if cfg.Debug {
						n.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
					}
					if cfg.EnableMetrics {
						jetStreamNumberOfFailSendMsgs.WithLabelValues(name, "marshal_error").Inc()
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
							jetStreamNumberOfFailSendMsgs.WithLabelValues(name, "template_error").Inc()
							continue
						}
					}

					subject, err = n.subjectName(r, m.GetMeta(), dc, cfg)
					if err != nil {
						if cfg.Debug {
							n.logger.Printf("%s failed to get subject name: %v", workerLogPrefix, err)
						}
						if cfg.EnableMetrics {
							jetStreamNumberOfFailSendMsgs.WithLabelValues(name, "subject_name_error").Inc()
						}
						continue
					}
					var start time.Time
					if cfg.EnableMetrics {
						start = time.Now()
					}
					_, err = js.Publish(subject, b, nats.Context(ctx))
					if err != nil {
						if cfg.Debug {
							n.logger.Printf("%s failed to write to subject '%s': %v", workerLogPrefix, subject, err)
						}
						if cfg.EnableMetrics {
							jetStreamNumberOfFailSendMsgs.WithLabelValues(name, "publish_error").Inc()
						}
						natsConn.Close()
						time.Sleep(cfg.ConnectTimeWait)
						goto CRCONN
					}
					if cfg.EnableMetrics {
						jetStreamSendDuration.WithLabelValues(name).Set(float64(time.Since(start).Nanoseconds()))
						jetStreamNumberOfSentMsgs.WithLabelValues(name, subject).Inc()
						jetStreamNumberOfSentBytes.WithLabelValues(name, subject).Add(float64(len(b)))
					}
				}
			}
		}
	}
}

type customDialer struct {
	ctx    context.Context
	logger *log.Logger
}

func (n *jetstreamOutput) newCustomDialer(ctx context.Context) *customDialer {
	return &customDialer{ctx: ctx, logger: n.logger}
}

func (d *customDialer) Dial(network, address string) (net.Conn, error) {
	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()

	d.logger.Printf("attempting to connect to %s", address)
	select {
	case <-d.ctx.Done():
		return nil, d.ctx.Err()
	default:
		nd := &net.Dialer{}
		conn, err := nd.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		d.logger.Printf("successfully connected to NATS server %s", address)
		return conn, nil
	}
}

func (n *jetstreamOutput) createNATSConn(ctx context.Context, c *config, idx int) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(fmt.Sprintf("%s-%d", c.Name, idx)),
		nats.SetCustomDialer(n.newCustomDialer(ctx)),
		nats.ReconnectWait(c.ConnectTimeWait),
		// nats.ReconnectBufSize(natsReconnectBufferSize),
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
	if c.Username != "" && c.Password != "" {
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	}
	nc, err := nats.Connect(c.Address, opts...)
	if err != nil {
		return nil, err
	}
	return nc, nil
}

func (n *jetstreamOutput) subjectName(m proto.Message, meta outputs.Meta, dc *dynConfig, cfg *config) (string, error) {
	sb := stringBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringBuilderPool.Put(sb)
	}()
	sb.WriteString(cfg.Stream)
	sb.WriteString(".")
	switch cfg.SubjectFormat {
	case subjectFormat_Static:
		sb.WriteString(cfg.Subject)
	case subjectFormat_TargetSub:
		err := dc.targetTpl.Execute(sb, meta)
		if err != nil {
			return "", err
		}
		if sub, ok := meta["subscription-name"]; ok {
			sb.WriteString(".")
			sb.WriteString(sub)
		}
	case subjectFormat_SubTarget:
		if sub, ok := meta["subscription-name"]; ok {
			sb.WriteString(sub)
			sb.WriteString(".")
		}
		err := dc.targetTpl.Execute(sb, meta)
		if err != nil {
			return "", err
		}
	case subjectFormat_SubTargetPath:
		if sub, ok := meta["subscription-name"]; ok {
			sb.WriteString(sub)
			sb.WriteString(".")
		}
		err := dc.targetTpl.Execute(sb, meta)
		if err != nil {
			return "", err
		}
		sb.WriteString(".")
		switch rsp := m.(type) {
		case *gnmi.SubscribeResponse:
			switch rsp := rsp.Response.(type) {
			case *gnmi.SubscribeResponse_Update:
				var prefixSubject string
				if rsp.Update.GetPrefix() != nil {
					prefixSubject = gNMIPathToSubject(rsp.Update.GetPrefix(), false)
				}
				var pathSubject string
				if len(rsp.Update.GetUpdate()) > 0 {
					pathSubject = gNMIPathToSubject(rsp.Update.GetUpdate()[0].GetPath(), false)
				}
				if prefixSubject != "" {
					sb.WriteString(prefixSubject)
					sb.WriteString(".")
				}
				if pathSubject != "" {
					sb.WriteString(pathSubject)
				}
			}
		}
	case subjectFormat_SubTargetPathWithKeys:
		if sub, ok := meta["subscription-name"]; ok {
			sb.WriteString(sub)
			sb.WriteString(".")
		}
		err := dc.targetTpl.Execute(sb, meta)
		if err != nil {
			return "", err
		}
		sb.WriteString(".")
		switch rsp := m.(type) {
		case *gnmi.SubscribeResponse:
			switch rsp := rsp.Response.(type) {
			case *gnmi.SubscribeResponse_Update:
				var prefixSubject string
				if rsp.Update.GetPrefix() != nil {
					prefixSubject = gNMIPathToSubject(rsp.Update.GetPrefix(), true)
				}
				var pathSubject string
				if len(rsp.Update.GetUpdate()) > 0 {
					pathSubject = gNMIPathToSubject(rsp.Update.GetUpdate()[0].GetPath(), true)
				}
				if prefixSubject != "" {
					sb.WriteString(prefixSubject)
					sb.WriteString(".")
				}
				if pathSubject != "" {
					sb.WriteString(pathSubject)
				}
			}
		}
	}
	return sb.String(), nil
}

func splitSubscribeResponse(m *gnmi.SubscribeResponse_Update) []proto.Message {
	if m == nil || m.Update == nil {
		return nil
	}
	rs := make([]proto.Message, 0, len(m.Update.GetUpdate())+len(m.Update.Delete))
	for _, upd := range m.Update.GetUpdate() {
		rs = append(rs, &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{
				Update: &gnmi.Notification{
					Timestamp: m.Update.GetTimestamp(),
					Prefix:    m.Update.GetPrefix(),
					Update:    []*gnmi.Update{upd},
				},
			},
		})
	}
	for _, del := range m.Update.GetDelete() {
		rs = append(rs, &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{
				Update: &gnmi.Notification{
					Timestamp: m.Update.GetTimestamp(),
					Prefix:    m.Update.GetPrefix(),
					Delete:    []*gnmi.Path{del},
				},
			},
		})
	}
	return rs
}

func gNMIPathToSubject(p *gnmi.Path, keys bool) string {
	if p == nil {
		return ""
	}
	sb := stringBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringBuilderPool.Put(sb)
	}()
	if p.GetOrigin() != "" {
		fmt.Fprintf(sb, "%s.", p.GetOrigin())
	}
	for i, e := range p.GetElem() {
		if i > 0 {
			sb.WriteString(".")
		}
		sb.WriteString(e.Name)
		if keys {
			if len(e.Key) > 0 {
				// sort keys by name
				kNames := make([]string, 0, len(e.Key))
				for k := range e.Key {
					kNames = append(kNames, k)
				}
				sort.Strings(kNames)
				for _, k := range kNames {
					sk := sanitizeKey(e.GetKey()[k])
					fmt.Fprintf(sb, ".{%s=%s}", k, sk)
				}
			}
		}
	}
	return sb.String()
}

var stringBuilderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

func sanitizeKey(k string) string {
	// Fast path: no special chars
	if !strings.ContainsAny(k, ". ") {
		return k
	}

	sb := stringBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringBuilderPool.Put(sb)
	}()

	sb.Grow(len(k))

	for _, r := range k {
		switch r {
		case '.':
			sb.WriteRune('^')
		case ' ':
			sb.WriteRune('~')
		default:
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

func storageType(s string) nats.StorageType {
	switch strings.ToLower(s) {
	case "file":
		return nats.FileStorage
	case "memory":
		return nats.MemoryStorage
	}
	return nats.MemoryStorage
}

func isValidRetentionPolicy(policy string) bool {
	switch strings.ToLower(policy) {
	case "limits", "workqueue":
		return true
	}
	return false
}

func retentionPolicy(s string) nats.RetentionPolicy {
	switch strings.ToLower(s) {
	case "workqueue":
		return nats.WorkQueuePolicy
	case "limits":
		return nats.LimitsPolicy
	}
	return nats.LimitsPolicy
}

// var storageTypes = map[string]nats.StorageType{
// 	"file":   nats.FileStorage,
// 	"memory": nats.MemoryStorage,
// }

func (n *jetstreamOutput) createStream(js nats.JetStreamContext, cfg *config) error {
	// If CreateStream is not configured, we're using an existing stream
	if cfg.CreateStream == nil {
		return nil
	}

	stream, err := js.StreamInfo(cfg.Stream)
	if err != nil {
		if !errors.Is(err, nats.ErrStreamNotFound) {
			return err
		}
	}

	// Stream exists, nothing to do
	if stream != nil {
		return nil
	}

	// Create stream with configured retention policy
	streamConfig := &nats.StreamConfig{
		Name:        cfg.Stream,
		Description: cfg.CreateStream.Description,
		Retention:   retentionPolicy(cfg.CreateStream.Retention),
		Subjects:    cfg.CreateStream.Subjects,
		Storage:     storageType(cfg.CreateStream.Storage),
		MaxMsgs:     cfg.CreateStream.MaxMsgs,
		MaxBytes:    cfg.CreateStream.MaxBytes,
		MaxAge:      cfg.CreateStream.MaxAge,
		MaxMsgSize:  cfg.CreateStream.MaxMsgSize,
	}
	// stream exists
	if stream != nil {
		_, err = js.UpdateStream(streamConfig)
		return err
	}

	_, err = js.AddStream(streamConfig)
	return err
}

func channelNeedsSwap(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.BufferSize != nw.BufferSize
}

func needsWorkerRestart(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.NumWorkers != nw.NumWorkers ||
		!old.TLS.Equal(nw.TLS) ||
		old.Address != nw.Address ||
		old.Username != nw.Username ||
		old.Password != nw.Password
}

func streamChanged(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	// stream name changed?
	if old.Stream != nw.Stream {
		return true
	}
	// create stream presence changed?
	if (old.CreateStream == nil) != (nw.CreateStream == nil) {
		return true
	}
	// both nil: nothing else to compare
	if old.CreateStream == nil && nw.CreateStream == nil {
		return false
	}
	// compare contents
	oc, nc := old.CreateStream, nw.CreateStream
	if oc.Description != nc.Description {
		return true
	}
	if !slices.Equal(oc.Subjects, nc.Subjects) {
		return true
	}
	if storageType(oc.Storage) != storageType(nc.Storage) {
		return true
	}
	if oc.MaxMsgs != nc.MaxMsgs {
		return true
	}
	if oc.MaxBytes != nc.MaxBytes {
		return true
	}
	if oc.MaxAge != nc.MaxAge {
		return true
	}
	if oc.MaxMsgSize != nc.MaxMsgSize {
		return true
	}
	return false
}
