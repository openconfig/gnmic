package cache

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/karimra/gnmic/utils"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

const (
	loggingPrefixNATS       = "[cache:nats] "
	cacheSubjects           = "gnmic.cache.subjects"
	subjectCacheResetPeriod = 30 * time.Second
)

type natsCache struct {
	cfg *Config
	oc  *gnmiCache

	ns  *server.Server
	nc  *nats.Conn
	cfn context.CancelFunc

	subjectChan chan string

	// configured remote address or locally started server address
	addr string

	m        *sync.RWMutex
	subjects map[string]struct{}
	logger   *log.Logger
}

func newNATSCache(cfg *Config, opts ...Option) (*natsCache, error) {
	if cfg == nil {
		cfg = new(Config)
	}
	cfg.setDefaults()

	var err error
	c := &natsCache{
		cfg:         cfg,
		oc:          newGNMICache(cfg, "nats", opts...),
		subjectChan: make(chan string),

		m:        new(sync.RWMutex),
		subjects: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.cfg.Address == defaultNATSAddress {
		sopts := &server.Options{
			Host:   cfg.Address,
			Port:   -1,
			NoSigs: true,
		}

		c.ns, err = server.NewServer(sopts)
		if err != nil {
			return nil, err
		}
	}
	if c.logger == nil {
		c.logger = log.New(os.Stderr, loggingPrefixNATS, utils.DefaultLoggingFlags)
	}
	c.start()
	ctx, cancel := context.WithCancel(context.Background())
	c.cfn = cancel
	go c.sync(ctx)
	return c, nil
}

func (c *natsCache) start() {
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
		nats.Timeout(c.cfg.Timeout),
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
}

func (c *natsCache) sync(ctx context.Context) {
	c.logger.Printf("start NATS sync")
	// this map keeps track of subjects already queued
	subjects := make(map[string]struct{})
	go func() {
		ticker := time.NewTicker(subjectCacheResetPeriod)
	START:
		subjectSub, err := c.nc.Subscribe(cacheSubjects, func(m *nats.Msg) {
			subj := string(m.Data)
			c.subjectChan <- subj
		})
		if err != nil {
			time.Sleep(time.Second)
			goto START
		}
		defer subjectSub.Unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.m.Lock()
				c.subjects = make(map[string]struct{})
				c.m.Unlock()
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case cc := <-c.subjectChan:
			if _, ok := subjects[cc]; !ok {
				c.logger.Printf("start NATS topic %q sync", cc)
				subjects[cc] = struct{}{}
				go c.syncSubject(ctx, cc)
			}
		}
	}
}

func (c *natsCache) syncSubject(ctx context.Context, subject string) {
START:
	sub, err := c.nc.Subscribe(fmt.Sprintf("%s.>", subject),
		func(msg *nats.Msg) {
			m := new(gnmi.SubscribeResponse)
			err := proto.Unmarshal([]byte(msg.Data), m)
			if err != nil {
				c.logger.Printf("failed to unmarshal proto msg: %v", err)
				return
			}
			c.oc.Write(ctx, subject, m)
		})
	if err != nil {
		time.Sleep(time.Second)
		goto START
	}
	defer sub.Unsubscribe()
	for range ctx.Done() {
	}
}

func (c *natsCache) Write(ctx context.Context, subscriptionName string, m proto.Message) {
	// write the msg to nats
	c.writeRemoteNATS(ctx, subscriptionName, m)
	// publish the subscription name to nats for other gnmic instances
	var ok bool
	c.m.RLock()
	defer func() {
		c.m.RUnlock()
		if !ok {
			c.m.Lock()
			c.subjects[subscriptionName] = struct{}{}
			c.m.Unlock()
			_ = c.nc.Publish(cacheSubjects, []byte(subscriptionName))
		}
	}()
	_, ok = c.subjects[subscriptionName]
}

func (c *natsCache) writeRemoteNATS(ctx context.Context, subscriptionName string, m proto.Message) {
	switch m := m.ProtoReflect().Interface().(type) {
	case *gnmi.SubscribeResponse:
		switch rsp := m.GetResponse().(type) {
		case *gnmi.SubscribeResponse_Update:
			targetName := rsp.Update.GetPrefix().GetTarget()
			if targetName == "" {
				c.logger.Printf("subscription=%q: response missing target: %v", subscriptionName, rsp)
				return
			}
			c.subjectChan <- subscriptionName
			var err error
			err = c.publishNotificationNATS(ctx, subscriptionName, targetName, m)
			if err != nil {
				c.logger.Print(err)
			}
		}
	}
}

func (c *natsCache) publishNotificationNATS(_ context.Context, subscriptionName, targetName string, r *gnmi.SubscribeResponse) error {
	b, err := proto.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal proto message: %w", err)
	}
	err = c.nc.Publish(fmt.Sprintf("%s.%s", subscriptionName, targetName), b)
	if err != nil {
		return fmt.Errorf("failed to publish to NATS cache: %w", err)
	}
	return nil
}

func (c *natsCache) Read() (map[string][]*gnmi.Notification, error) {
	return c.oc.Read()
}

func (c *natsCache) Subscribe(ctx context.Context, ro *ReadOpts) chan *Notification {
	return c.oc.Subscribe(ctx, ro)
}

func (c *natsCache) Stop() {
	c.cfn()
	if c.nc != nil {
		c.nc.Close()
	}
	if c.ns != nil {
		c.ns.Shutdown()
	}
}

func (c *natsCache) SetLogger(logger *log.Logger) {
	if logger != nil && c.logger != nil {
		c.logger.SetOutput(logger.Writer())
		c.logger.SetFlags(logger.Flags())
		c.logger.SetPrefix(loggingPrefixNATS)
	}
}

func (c *natsCache) DeleteTarget(name string) {
	c.oc.DeleteTarget(name)
}
