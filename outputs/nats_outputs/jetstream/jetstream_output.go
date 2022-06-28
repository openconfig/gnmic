package jetstream_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/nats-io/nats.go"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
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
		return &jetstreamOutput{
			Cfg:     &config{},
			msgChan: make(chan *outputs.ProtoMsg),
			wg:      new(sync.WaitGroup),
			logger:  log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type subjectFormat string

const (
	subjectFormat_Static                = "static"
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
	TLS                *tls                `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	Format             string              `mapstructure:"format,omitempty" json:"format,omitempty"`
	AddTarget          string              `mapstructure:"add-target,omitempty" json:"add-target,omitempty"`
	TargetTemplate     string              `mapstructure:"target-template,omitempty" json:"target-template,omitempty"`
	MsgTemplate        string              `mapstructure:"msg-template,omitempty" json:"msg-template,omitempty"`
	OverrideTimestamps bool                `mapstructure:"override-timestamps,omitempty" json:"override-timestamps,omitempty"`
	NumWorkers         int                 `mapstructure:"num-workers,omitempty" json:"num-workers,omitempty"`
	WriteTimeout       time.Duration       `mapstructure:"write-timeout,omitempty" json:"write-timeout,omitempty"`
	Debug              bool                `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	EnableMetrics      bool                `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	EventProcessors    []string            `mapstructure:"event-processors,omitempty" json:"event-processors,omitempty"`
}

type createStreamConfig struct {
	Description string        `mapstructure:"description,omitempty" json:"description,omitempty"`
	Subjects    []string      `mapstructure:"subjects,omitempty" json:"subjects,omitempty"`
	Storage     string        `mapstructure:"storage,omitempty" json:"storage,omitempty"`
	MaxMsgs     int64         `mapstructure:"max-msgs,omitempty" json:"max-msgs,omitempty"`
	MaxBytes    int64         `mapstructure:"max-bytes,omitempty" json:"max-bytes,omitempty"`
	MaxAge      time.Duration `mapstructure:"max-age,omitempty" json:"max-age,omitempty"`
	MaxMsgSize  int32         `mapstructure:"max-msg-size,omitempty" json:"max-msg-size,omitempty"`
}

type tls struct {
	CAFile     string `mapstructure:"ca-file,omitempty" json:"ca-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty" json:"cert-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty" json:"key-file,omitempty"`
	SkipVerify bool   `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty"`
}

// jetstreamOutput //
type jetstreamOutput struct {
	Cfg      *config
	ctx      context.Context
	cancelFn context.CancelFunc
	msgChan  chan *outputs.ProtoMsg
	wg       *sync.WaitGroup
	logger   *log.Logger
	mo       *formatters.MarshalOptions
	evps     []formatters.EventProcessor

	targetTpl *template.Template
	msgTpl    *template.Template
}

func (n *jetstreamOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
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

	n.logger.SetPrefix(fmt.Sprintf(loggingPrefix, n.Cfg.Name))
	n.msgChan = make(chan *outputs.ProtoMsg)
	initMetrics()
	n.mo = &formatters.MarshalOptions{
		Format:     n.Cfg.Format,
		OverrideTS: n.Cfg.OverrideTimestamps,
	}
	if n.Cfg.TargetTemplate == "" {
		n.targetTpl = outputs.DefaultTargetTemplate
	} else if n.Cfg.AddTarget != "" {
		n.targetTpl, err = utils.CreateTemplate("target-template", n.Cfg.TargetTemplate)
		if err != nil {
			return err
		}
		n.targetTpl = n.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	if n.Cfg.MsgTemplate != "" {
		n.msgTpl, err = utils.CreateTemplate("msg-template", n.Cfg.MsgTemplate)
		if err != nil {
			return err
		}
		n.msgTpl = n.msgTpl.Funcs(outputs.TemplateFuncs)
	}

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

func (n *jetstreamOutput) setDefaults() error {
	if n.Cfg.Stream == "" {
		return errors.New("missing stream name")
	}
	if n.Cfg.Format == "" {
		n.Cfg.Format = defaultFormat
	}
	if n.Cfg.SubjectFormat == "" {
		n.Cfg.SubjectFormat = subjectFormat_Static
	}
	switch n.Cfg.SubjectFormat {
	case subjectFormat_Static,
		subjectFormat_SubTarget,
		subjectFormat_SubTargetPath,
		subjectFormat_SubTargetPathWithKeys:
	default:
		return fmt.Errorf("unknown subject-format value: %v", n.Cfg.SubjectFormat)
	}
	if n.Cfg.Subject == "" {
		n.Cfg.Subject = defaultSubjectName
	}
	if n.Cfg.Address == "" {
		n.Cfg.Address = defaultAddress
	}
	if n.Cfg.ConnectTimeWait <= 0 {
		n.Cfg.ConnectTimeWait = natsConnectWait
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
	if n.Cfg.CreateStream != nil {
		if len(n.Cfg.CreateStream.Subjects) == 0 {
			n.Cfg.CreateStream.Subjects = []string{fmt.Sprintf("%s.>", n.Cfg.Stream)}
		}
		if n.Cfg.CreateStream.Description == "" {
			n.Cfg.CreateStream.Description = "created by gNMIc"
		}
		if n.Cfg.CreateStream.Storage == "" {
			n.Cfg.CreateStream.Storage = "memory"
		}
		return nil
	}
	return nil
}

func (n *jetstreamOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
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
			n.logger.Printf("writing expired after %s, JetStream output might not be initialized", n.Cfg.WriteTimeout)
		}
		if n.Cfg.EnableMetrics {
			jetStreamNumberOfFailSendMsgs.WithLabelValues(n.Cfg.Name, "timeout").Inc()
		}
		return
	}
}

func (n *jetstreamOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

func (n *jetstreamOutput) Close() error {
	n.cancelFn()
	n.wg.Wait()
	return nil
}

func (n *jetstreamOutput) RegisterMetrics(reg *prometheus.Registry) {
	if !n.Cfg.EnableMetrics {
		return
	}
	if err := registerMetrics(reg); err != nil {
		n.logger.Printf("failed to register metric: %+v", err)
	}
}

func (c *config) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}

func (n *jetstreamOutput) String() string {
	b, err := json.Marshal(n)
	if err != nil {
		return ""
	}
	return string(b)
}

func (n *jetstreamOutput) SetLogger(logger *log.Logger) {
	if logger != nil && n.logger != nil {
		n.logger.SetOutput(logger.Writer())
		n.logger.SetFlags(logger.Flags())
	}
}

func (n *jetstreamOutput) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) {
	for _, epName := range n.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType],
					formatters.WithLogger(logger),
					formatters.WithTargets(tcs),
					formatters.WithActions(acts))
				if err != nil {
					n.logger.Printf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
					continue
				}
				n.evps = append(n.evps, ep)
				n.logger.Printf("added event processor '%s' of type=%s to jetstream output", epName, epType)
				continue
			}
			n.logger.Printf("%q event processor has an unknown type=%q", epName, epType)
			continue
		}
		n.logger.Printf("%q event processor not found!", epName)
	}
}

func (n *jetstreamOutput) SetName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(n.Cfg.Name)
	n.Cfg.Name = sb.String()
}

func (n *jetstreamOutput) SetClusterName(string) {}

func (n *jetstreamOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}

func (n *jetstreamOutput) worker(ctx context.Context, i int, cfg *config) {
	defer n.wg.Done()
	var natsConn *nats.Conn
	var err error
	var subject string
	workerLogPrefix := fmt.Sprintf("worker-%d", i)
	n.logger.Printf("%s starting", workerLogPrefix)
CRCONN:
	natsConn, err = n.createNATSConn(cfg)
	if err != nil {
		n.logger.Printf("%s failed to create connection: %v", workerLogPrefix, err)
		time.Sleep(n.Cfg.ConnectTimeWait)
		goto CRCONN
	}
	defer natsConn.Close()
	js, err := natsConn.JetStream()
	if err != nil {
		if n.Cfg.Debug {
			n.logger.Printf("%s failed to create jetstream context: %v", workerLogPrefix, err)
		}
		if n.Cfg.EnableMetrics {
			jetStreamNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "jetstream_context_error").Inc()
		}
		natsConn.Close()
		time.Sleep(cfg.ConnectTimeWait)
		goto CRCONN
	}
	n.logger.Printf("%s initialized nats jetstream producer: %s", workerLogPrefix, cfg)
	// worker-0 create stream if configured
	if i == 0 {
		err = n.createStream(js)
		if err != nil {
			if n.Cfg.Debug {
				n.logger.Printf("%s failed to create stream: %v", workerLogPrefix, err)
			}
			if n.Cfg.EnableMetrics {
				jetStreamNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "create_stream_error").Inc()
			}
			natsConn.Close()
			time.Sleep(cfg.ConnectTimeWait)
			goto CRCONN
		}
	}
	for {
		select {
		case <-ctx.Done():
			n.logger.Printf("%s shutting down", workerLogPrefix)
			return
		case m := <-n.msgChan:
			err = outputs.AddSubscriptionTarget(m.GetMsg(), m.GetMeta(), n.Cfg.AddTarget, n.targetTpl)
			if err != nil {
				n.logger.Printf("failed to add target to the response: %v", err)
			}
			var rs []proto.Message
			switch n.Cfg.SubjectFormat {
			case subjectFormat_Static, subjectFormat_SubTarget:
				rs = []proto.Message{m.GetMsg()}
			case subjectFormat_SubTargetPath, subjectFormat_SubTargetPathWithKeys:
				switch rsp := m.GetMsg().(type) {
				case *gnmi.SubscribeResponse:
					switch rsp := rsp.Response.(type) {
					case *gnmi.SubscribeResponse_Update:
						rs = splitSubscribeResponse(rsp)
					}
				}
			}
			for _, r := range rs {
				b, err := n.mo.Marshal(r, m.GetMeta(), n.evps...)
				if err != nil {
					if n.Cfg.Debug {
						n.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
					}
					if n.Cfg.EnableMetrics {
						jetStreamNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "marshal_error").Inc()
					}
					continue
				}

				if n.msgTpl != nil && len(b) > 0 {
					b, err = outputs.ExecTemplate(b, n.msgTpl)
					if err != nil {
						if n.Cfg.Debug {
							log.Printf("failed to execute template: %v", err)
						}
						jetStreamNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "template_error").Inc()
						return
					}
				}

				subject, err = n.subjectName(r, m.GetMeta())
				if err != nil {
					if n.Cfg.Debug {
						n.logger.Printf("%s failed to get subject name: %v", workerLogPrefix, err)
					}
					if n.Cfg.EnableMetrics {
						jetStreamNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "subject_name_error").Inc()
					}
					continue
				}
				var start time.Time
				if n.Cfg.EnableMetrics {
					start = time.Now()
				}
				_, err = js.Publish(subject, b)
				if err != nil {
					if n.Cfg.Debug {
						n.logger.Printf("%s failed to write to subject '%s': %v", workerLogPrefix, subject, err)
					}
					if n.Cfg.EnableMetrics {
						jetStreamNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "publish_error").Inc()
					}
					natsConn.Close()
					time.Sleep(cfg.ConnectTimeWait)
					goto CRCONN
				}
				if n.Cfg.EnableMetrics {
					jetStreamSendDuration.WithLabelValues(cfg.Name).Set(float64(time.Since(start).Nanoseconds()))
					jetStreamNumberOfSentMsgs.WithLabelValues(cfg.Name, subject).Inc()
					jetStreamNumberOfSentBytes.WithLabelValues(cfg.Name, subject).Add(float64(len(b)))
				}
			}
		}
	}
}

// Dial //
func (n *jetstreamOutput) Dial(network, address string) (net.Conn, error) {
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

func (n *jetstreamOutput) createNATSConn(c *config) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(c.Name),
		nats.SetCustomDialer(n),
		nats.ReconnectWait(n.Cfg.ConnectTimeWait),
		// nats.ReconnectBufSize(natsReconnectBufferSize),
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
	if n.Cfg.TLS != nil {
		tlsConfig, err := utils.NewTLSConfig(
			n.Cfg.TLS.CAFile, n.Cfg.TLS.CertFile, n.Cfg.TLS.KeyFile, n.Cfg.TLS.SkipVerify,
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

func (n *jetstreamOutput) subjectName(m proto.Message, meta outputs.Meta) (string, error) {
	sb := new(strings.Builder)
	sb.WriteString(n.Cfg.Stream)
	sb.WriteString(".")
	switch n.Cfg.SubjectFormat {
	case subjectFormat_Static:
		sb.WriteString(n.Cfg.Subject)
	case subjectFormat_SubTarget:
		if sub, ok := meta["subscription-name"]; ok {
			sb.WriteString(sub)
			sb.WriteString(".")
		}
		err := n.targetTpl.Execute(sb, meta)
		if err != nil {
			return "", err
		}
	case subjectFormat_SubTargetPath:
		if sub, ok := meta["subscription-name"]; ok {
			sb.WriteString(sub)
			sb.WriteString(".")
		}
		err := n.targetTpl.Execute(sb, meta)
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
		err := n.targetTpl.Execute(sb, meta)
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
	sb := new(strings.Builder)
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

const (
	dotReplChar   = "^"
	spaceReplChar = "~"
)

var regDot = regexp.MustCompile(`\.`)
var regSpace = regexp.MustCompile(`\s`)

func sanitizeKey(k string) string {
	s := regDot.ReplaceAllString(k, dotReplChar)
	return regSpace.ReplaceAllString(s, spaceReplChar)
}

var storageTypes = map[string]nats.StorageType{
	"file":   nats.FileStorage,
	"memory": nats.MemoryStorage,
}

func (n *jetstreamOutput) createStream(js nats.JetStreamContext) error {
	if n.Cfg.CreateStream == nil {
		return nil
	}
	stream, err := js.StreamInfo(n.Cfg.Stream)
	if err != nil {
		if !errors.Is(err, nats.ErrStreamNotFound) {
			return err
		}
	}
	// stream exists
	if stream != nil {
		return nil
	}
	// create stream
	streamConfig := &nats.StreamConfig{
		Name:        n.Cfg.Stream,
		Description: n.Cfg.CreateStream.Description,
		Subjects:    n.Cfg.CreateStream.Subjects,
		Storage:     storageTypes[strings.ToLower(n.Cfg.CreateStream.Storage)],
		MaxMsgs:     n.Cfg.CreateStream.MaxMsgs,
		MaxBytes:    n.Cfg.CreateStream.MaxBytes,
		MaxAge:      n.Cfg.CreateStream.MaxAge,
		MaxMsgSize:  n.Cfg.CreateStream.MaxMsgSize,
	}
	_, err = js.AddStream(streamConfig)
	return err
}
