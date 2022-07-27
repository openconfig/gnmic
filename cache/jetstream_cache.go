package cache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/karimra/gnmic/utils"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

const (
	loggingPrefixJetStream = "[cache:jetstream] "
	reconnectTimer         = 5 * time.Second
	defaultFetchBatchSize  = 100
	defaultFetchWaitTime   = 100 * time.Millisecond
	defaultExpiration      = time.Minute
	defaultMaxMsgs         = 1024 * 1024
	defaultMaxBytes        = 1024 * 1024 * 1024
	defaultNATSAddress     = "127.0.0.1"
	jetStreamSyncName      = "gnmic-jetstream-cache"
)

type jetStreamCache struct {
	cfg        *Config
	ns         *server.Server
	nc         *nats.Conn
	js         nats.JetStreamContext
	cfn        context.CancelFunc
	streamChan chan string

	// configured remote address or locally started server address
	addr string
	oc   *gnmiCache

	m       *sync.RWMutex
	streams map[string]struct{}
	logger  *log.Logger
}

func newJetStreamCache(cfg *Config, opts ...Option) (*jetStreamCache, error) {
	if cfg == nil {
		cfg = new(Config)
	}
	cfg.setDefaults()

	var err error
	c := &jetStreamCache{
		cfg:        cfg,
		oc:         newGNMICache(cfg, "jetstream", opts...),
		streamChan: make(chan string),
		m:          new(sync.RWMutex),
		streams:    make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.cfg.Address == defaultNATSAddress {
		sopts := &server.Options{
			Host:      cfg.Address,
			Port:      -1,
			JetStream: true,
			NoSigs:    true,
		}

		c.ns, err = server.NewServer(sopts)
		if err != nil {
			return nil, err
		}
	}
	if c.logger == nil {
		c.logger = log.New(os.Stderr, loggingPrefixJetStream, utils.DefaultLoggingFlags)
	}
	c.start()
	ctx, cancel := context.WithCancel(context.Background())
	c.cfn = cancel
	go c.sync(ctx)
	return c, nil
}

func (c *jetStreamCache) SetLogger(logger *log.Logger) {
	if logger != nil && c.logger != nil {
		c.logger.SetOutput(logger.Writer())
		c.logger.SetFlags(logger.Flags())
		c.logger.SetPrefix(loggingPrefixJetStream)
	}
}

func (c *jetStreamCache) start() {
START:
	if c.ns != nil {
		go c.ns.Start()
		if !c.ns.ReadyForConnections(reconnectTimer) {
			c.ns.Shutdown()
			c.logger.Printf("failed to start cache, retrying")
			goto START
		}
	}

	c.addr = c.cfg.Address
	if c.ns != nil {
		c.addr = c.ns.ClientURL()
	}

	var err error
	opts := []nats.Option{
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			c.logger.Printf("NATS error: %v", err)
		}),
		nats.DisconnectHandler(func(_ *nats.Conn) {
			c.logger.Println("Disconnected from NATS")
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			c.logger.Println("NATS connection is closed")
		}),
	}
	if c.cfg.Username != "" && c.cfg.Password != "" {
		opts = append(opts, nats.UserInfo(c.cfg.Username, c.cfg.Password))
	}
CONNECT:
	if c.nc != nil {
		c.nc.Close()
	}

	c.nc, err = nats.Connect(c.addr, opts...)
	if err != nil {
		c.logger.Printf("failed to connect: %v", err)
		time.Sleep(reconnectTimer)
		goto CONNECT
	}

	c.js, err = c.nc.JetStream()
	if err != nil {
		c.logger.Printf("failed to create stream: %v", err)
		time.Sleep(reconnectTimer)
		goto CONNECT
	}
}

func (c *jetStreamCache) createStream(streamName string, subjects []string) error {
	stream, err := c.js.StreamInfo(streamName)
	if err != nil {
		if !errors.Is(err, nats.ErrStreamNotFound) {
			return err
		}
	}
	if c.cfg.Debug {
		c.logger.Printf("found stream %q: %v", streamName, stream != nil)
	}
	if stream == nil {
		c.logger.Printf("creating stream %q and subjects %q", streamName, subjects)
		_, err = c.js.AddStream(
			&nats.StreamConfig{
				Name:     streamName,
				Subjects: subjects,
				MaxMsgs:  c.cfg.MaxMsgsPerSubscription,
				MaxBytes: c.cfg.MaxBytes,
				Discard:  nats.DiscardOld,
				MaxAge:   c.cfg.Expiration,
				Storage:  nats.MemoryStorage,
			})
		return err
	}
	return nil
}

func (c *jetStreamCache) Write(ctx context.Context, subscriptionName string, m proto.Message) {
	switch m := m.ProtoReflect().Interface().(type) {
	case *gnmi.SubscribeResponse:
		switch rsp := m.GetResponse().(type) {
		case *gnmi.SubscribeResponse_Update:
			targetName := rsp.Update.GetPrefix().GetTarget()
			if targetName == "" {
				c.logger.Printf("subscription=%q: response missing target: %v", subscriptionName, rsp)
				return
			}

			// check if a stream with the same name as the subscription is being created or has been created
			c.m.RLock()
			_, ok := c.streams[subscriptionName]
			c.m.RUnlock()
			if !ok {
				// add the subscription as a stream and create it in NATS if it doesn't exist
				c.m.Lock()
				c.streams[subscriptionName] = struct{}{}
				err := c.createStream(subscriptionName, []string{fmt.Sprintf("%s.>", subscriptionName)})
				if err != nil {
					delete(c.streams, subscriptionName)
					c.m.Unlock()
					c.logger.Printf("failed to create stream: %v", err)
					return
				}
				c.m.Unlock()
				c.streamChan <- subscriptionName
			}

			// wait in case the stream is being created
			c.m.RLock()
			defer c.m.RUnlock()
			var err error
			for _, r := range splitSubscribeResponse(rsp.Update) {
				err = c.publishNotificationJS(ctx, subscriptionName, targetName, r)
				if err != nil {
					c.logger.Print(err)
				}
			}
		}
	}
}

func (c *jetStreamCache) publishNotificationJS(ctx context.Context, subscriptionName, targetName string, r *gnmi.SubscribeResponse) error {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	subjectName, err := subjectName(subscriptionName, targetName, r)
	if err != nil {
		return fmt.Errorf("failed to build a subject name: %w", err)
	}

	b, err := proto.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal proto message: %w", err)
	}

	_, err = c.js.Publish(subjectName, b, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("failed to publish to JetStream cache: %w", err)
	}
	return nil
}

func (c *jetStreamCache) sync(ctx context.Context) {
	c.logger.Printf("start JetStream sync")
	// this map keeps track of streams already queued
	streams := make(map[string]struct{})
	go func() {
	START:
		subjectSub, err := c.nc.Subscribe(cacheSubjects, func(m *nats.Msg) {
			subj := string(m.Data)
			c.streamChan <- subj
		})
		if err != nil {
			time.Sleep(time.Second)
			goto START
		}
		defer subjectSub.Unsubscribe()
		for range ctx.Done() {
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case cc := <-c.streamChan:
			if _, ok := streams[cc]; !ok {
				c.logger.Printf("start JetStream stream %q sync", cc)
				streams[cc] = struct{}{}
				go c.syncStream(ctx, cc)
			}
		}
	}
}

func (c *jetStreamCache) syncStream(ctx context.Context, subject string) {
START:
	sub, err := c.js.Subscribe(fmt.Sprintf("%s.>", subject),
		func(msg *nats.Msg) {
			m := new(gnmi.SubscribeResponse)
			err := proto.Unmarshal([]byte(msg.Data), m)
			if err != nil {
				c.logger.Printf("failed to unmarshal proto msg: %v", err)
				return
			}
			_ = msg.Ack()
			c.oc.Write(ctx, subject, m)
		},
		nats.DeliverNew(),
		nats.Durable(jetStreamSyncName),
	)
	if err != nil {
		time.Sleep(time.Second)
		goto START
	}
	defer sub.Unsubscribe()
	for range ctx.Done() {
	}
}

// Read //
func (c *jetStreamCache) Read(ctx context.Context, name string, ro *ReadOpts) (map[string][]*gnmi.Notification, error) {
	return c.oc.Read(ctx, name, ro)
}

func (c *jetStreamCache) Subscribe(ctx context.Context, ro *ReadOpts) chan *notification {
	if ro == nil {
		ro = &ReadOpts{
			Target:         "*",
			Paths:          []*gnmi.Path{{}},
			Mode:           "stream_sample",
			SampleInterval: 10 * time.Second,
		}
	}
	ch := make(chan *notification)
	go c.subscribe(ctx, ro, ch)
	return ch
}

func (c *jetStreamCache) subscribe(ctx context.Context, ro *ReadOpts, ch chan *notification) {
}

func (c *jetStreamCache) Stop() {
	c.cfn()
	if c.nc != nil {
		c.nc.Close()
	}
	if c.ns != nil {
		c.ns.Shutdown()
	}
}

func subjectName(streamName, target string, m proto.Message) (string, error) {
	sb := &strings.Builder{}
	sb.WriteString(streamName)
	sb.WriteString(".")
	if target != "" {
		sb.WriteString(target)
		sb.WriteString(".")
	}

	switch rsp := m.(type) {
	case *gnmi.SubscribeResponse:
		switch rsp := rsp.Response.(type) {
		case *gnmi.SubscribeResponse_Update:
			var prefixSubject string
			if rsp.Update.GetPrefix() != nil {
				prefixSubject = gNMIPathToSubject(rsp.Update.GetPrefix(), subjectOpts{WithKeys: true, WithWildcard: false})[0]
			}
			var pathSubject string
			if len(rsp.Update.GetUpdate()) > 0 {
				pathSubject = gNMIPathToSubject(rsp.Update.GetUpdate()[0].GetPath(), subjectOpts{WithKeys: true, WithWildcard: false})[0]
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
	return sb.String(), nil
}

func splitSubscribeResponse(m *gnmi.Notification) []*gnmi.SubscribeResponse {
	if m == nil {
		return nil
	}
	rs := make([]*gnmi.SubscribeResponse, 0, len(m.GetUpdate())+len(m.GetDelete()))
	for _, upd := range m.GetUpdate() {
		rs = append(rs, &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{
				Update: &gnmi.Notification{
					Timestamp: m.GetTimestamp(),
					Prefix:    m.GetPrefix(),
					Update:    []*gnmi.Update{upd},
				},
			},
		})
	}
	for _, del := range m.GetDelete() {
		rs = append(rs, &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{
				Update: &gnmi.Notification{
					Timestamp: m.GetTimestamp(),
					Prefix:    m.GetPrefix(),
					Delete:    []*gnmi.Path{del},
				},
			},
		})
	}
	return rs
}

type subjectOpts struct {
	WithKeys     bool
	WithWildcard bool
}

func gNMIPathToSubject(p *gnmi.Path, opts subjectOpts) []string {
	if p == nil {
		return []string{""}
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
		if opts.WithKeys {
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

	subj := sb.String()

	if subj == "" && opts.WithWildcard {
		return []string{".>"}
	}
	result := []string{subj}
	if opts.WithWildcard {
		result = append(result, subj+".>")
	}
	return result
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
