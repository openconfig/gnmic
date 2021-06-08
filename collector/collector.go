package collector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fullstorydev/grpcurl"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/jhump/protoreflect/desc"
	"github.com/karimra/gnmic/inputs"
	"github.com/karimra/gnmic/lockers"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

const (
	defaultTargetReceivebuffer = 1000
	defaultLockRetry           = 5 * time.Second
)

// Config is the collector config
type Config struct {
	Name                string
	PrometheusAddress   string
	Debug               bool
	Format              string
	TargetReceiveBuffer uint
	RetryTimer          time.Duration
	ClusterName         string
	LockRetryTimer      time.Duration
}

// Collector //
type Collector struct {
	Config   *Config
	dialOpts []grpc.DialOption
	//
	m             *sync.Mutex
	Subscriptions map[string]*SubscriptionConfig

	outputsConfig map[string]map[string]interface{}
	Outputs       map[string]outputs.Output

	inputsConfig map[string]map[string]interface{}
	Inputs       map[string]inputs.Input

	locker lockers.Locker

	targetsConfig map[string]*TargetConfig
	Targets       map[string]*Target

	EventProcessorsConfig map[string]map[string]interface{}
	logger                *log.Logger
	httpServer            *http.Server
	reg                   *prometheus.Registry

	targetsChan    chan *Target
	activeTargets  map[string]struct{}
	targetsLocksFn map[string]context.CancelFunc

	rootDesc desc.Descriptor
}

type CollectorOption func(c *Collector)

func WithLogger(logger *log.Logger) CollectorOption {
	return func(c *Collector) {
		if logger == nil {
			c.logger = log.New(ioutil.Discard, "", 0)
		} else {
			c.logger = logger
		}
	}
}

func WithSubscriptions(subs map[string]*SubscriptionConfig) CollectorOption {
	return func(c *Collector) {
		c.Subscriptions = subs
	}
}

func WithOutputs(outs map[string]map[string]interface{}) CollectorOption {
	return func(c *Collector) {
		for outputName, outputCfg := range outs {
			c.AddOutput(outputName, outputCfg)
		}
	}
}

func WithDialOptions(dialOptions []grpc.DialOption) CollectorOption {
	return func(c *Collector) {
		c.dialOpts = dialOptions
	}
}

func WithEventProcessors(eps map[string]map[string]interface{}) CollectorOption {
	return func(c *Collector) {
		c.EventProcessorsConfig = eps
	}
}

func WithProtoDescriptor(d desc.Descriptor) CollectorOption {
	return func(c *Collector) {
		c.rootDesc = d
	}
}

// NewCollector //
func NewCollector(config *Config, targetConfigs map[string]*TargetConfig, opts ...CollectorOption) *Collector {
	var httpServer *http.Server
	if config.TargetReceiveBuffer == 0 {
		config.TargetReceiveBuffer = defaultTargetReceivebuffer
	}
	if config.RetryTimer == 0 {
		config.RetryTimer = defaultRetryTimer
	}
	if config.LockRetryTimer <= 0 {
		config.LockRetryTimer = defaultLockRetry
	}
	c := &Collector{
		Config:         config,
		m:              new(sync.Mutex),
		targetsConfig:  make(map[string]*TargetConfig),
		Targets:        make(map[string]*Target),
		Outputs:        make(map[string]outputs.Output),
		Inputs:         make(map[string]inputs.Input),
		httpServer:     httpServer,
		targetsChan:    make(chan *Target),
		activeTargets:  make(map[string]struct{}),
		targetsLocksFn: make(map[string]context.CancelFunc),
	}
	for _, op := range opts {
		op(c)
	}
	if config.Debug {
		c.logger.Printf("starting collector with cfg=%+v", config)
	}
	if config.PrometheusAddress != "" {
		grpcMetrics := grpc_prometheus.NewClientMetrics()
		c.reg = prometheus.NewRegistry()
		c.reg.MustRegister(prometheus.NewGoCollector())
		c.reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
		grpcMetrics.EnableClientHandlingTimeHistogram()
		c.reg.MustRegister(grpcMetrics)
		handler := http.NewServeMux()
		handler.Handle("/metrics", promhttp.HandlerFor(c.reg, promhttp.HandlerOpts{}))
		c.httpServer = &http.Server{
			Handler: handler,
			Addr:    config.PrometheusAddress,
		}
		c.dialOpts = append(c.dialOpts, grpc.WithStreamInterceptor(grpcMetrics.StreamClientInterceptor()))
	}

	for _, tc := range targetConfigs {
		c.AddTarget(tc)
	}
	return c
}

// AddTarget initializes a target based on *TargetConfig
func (c *Collector) AddTarget(tc *TargetConfig) error {
	c.logger.Printf("adding target %+v", tc)
	if c.Targets == nil {
		c.Targets = make(map[string]*Target)
	}
	// if _, ok := c.Targets[tc.Name]; ok {
	// 	return fmt.Errorf("target %q already exists", tc.Name)
	// }
	if tc.BufferSize == 0 {
		tc.BufferSize = c.Config.TargetReceiveBuffer
	}
	if tc.RetryTimer == 0 {
		tc.RetryTimer = c.Config.RetryTimer
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.targetsConfig[tc.Name] = tc
	return nil
}

func (c *Collector) initTarget(name string) error {
	c.m.Lock()
	defer c.m.Unlock()
	if tc, ok := c.targetsConfig[name]; ok {
		if _, ok := c.Targets[name]; !ok {
			t := NewTarget(tc)
			//
			t.Subscriptions = make(map[string]*SubscriptionConfig)
			for _, subName := range tc.Subscriptions {
				if sub, ok := c.Subscriptions[subName]; ok {
					t.Subscriptions[subName] = sub
				}
			}
			if len(t.Subscriptions) == 0 {
				for _, sub := range c.Subscriptions {
					t.Subscriptions[sub.Name] = sub
				}
			}
			err := c.parseProtoFiles(t)
			if err != nil {
				return err
			}
			c.Targets[t.Config.Name] = t
		}
		return nil
	}
	return fmt.Errorf("unknown target")
}

func (c *Collector) CreateTarget(name string) error {
	c.m.Lock()
	defer c.m.Unlock()
	if tc, ok := c.targetsConfig[name]; ok {
		if _, ok := c.Targets[name]; !ok {
			c.Targets[tc.Name] = NewTarget(tc)
		}
		return nil
	}
	return fmt.Errorf("unknown target %q", name)
}

func (c *Collector) TargetSubscribeStream(ctx context.Context, name string) {
	lockKey := c.lockKey(name)
START:
	nctx, cancel := context.WithCancel(ctx)
	c.m.Lock()
	if cfn, ok := c.targetsLocksFn[name]; ok {
		cfn()
	}
	c.targetsLocksFn[name] = cancel
	c.m.Unlock()
	select {
	// check if the context was canceled before retrying
	case <-nctx.Done():
		return
	default:
		err := c.initTarget(name)
		if err != nil {
			c.logger.Printf("failed to initialize target %q: %v", name, err)
			return
		}
		select {
		case <-nctx.Done():
			return
		default:
			if c.locker != nil {
				c.logger.Printf("acquiring lock for target %q", name)
				ok, err := c.locker.Lock(nctx, lockKey, []byte(c.Config.Name))
				if err == lockers.ErrCanceled {
					c.logger.Printf("lock attempt for target %q canceled", name)
					return
				}
				if err != nil {
					c.logger.Printf("failed to lock target %q: %v", name, err)
					time.Sleep(c.Config.LockRetryTimer)
					goto START
				}
				if !ok {
					time.Sleep(c.Config.LockRetryTimer)
					goto START
				}
				c.logger.Printf("acquired lock for target %q", name)
			}
			select {
			case <-nctx.Done():
				return
			default:
				c.m.Lock()
				t := c.Targets[name]
				c.m.Unlock()
				c.targetsChan <- t
				c.logger.Printf("queuing target %q", name)
			}
			c.logger.Printf("subscribing to target: %q", name)
			go func() {
				err := c.Subscribe(nctx, name)
				if err != nil {
					c.logger.Printf("failed to subscribe: %v", err)
					return
				}
			}()
			if c.locker != nil {
				doneChan, errChan := c.locker.KeepLock(nctx, lockKey)
				for {
					select {
					case <-nctx.Done():
						c.logger.Printf("target %q stopped: %v", name, ctx.Err())
						return
					case <-doneChan:
						c.logger.Printf("target lock %q removed", name)
						return
					case err := <-errChan:
						c.logger.Printf("failed to maintain target %q lock: %v", name, err)
						c.StopTarget(ctx, name)
						if errors.Is(err, context.Canceled) {
							return
						}
						time.Sleep(c.Config.LockRetryTimer)
						goto START
					}
				}
			}
		}
	}
}

func (c *Collector) TargetSubscribeOnce(ctx context.Context, name string) error {
	nctx, cancel := context.WithCancel(ctx)
	defer cancel()
	err := c.initTarget(name)
	if err != nil {
		c.logger.Printf("failed to initialize target %q: %v", name, err)
		return err
	}
	c.logger.Printf("subscribing to target: %q", name)
	err = c.SubscribeOnce(nctx, name)
	if err != nil {
		c.logger.Printf("failed to subscribe: %v", err)
		return err
	}
	return nil
}

func (c *Collector) TargetSubscribePoll(ctx context.Context, name string) {
	nctx, cancel := context.WithCancel(ctx)
	c.m.Lock()
	if cfn, ok := c.targetsLocksFn[name]; ok {
		cfn()
	}
	c.targetsLocksFn[name] = cancel
	c.m.Unlock()
	err := c.initTarget(name)
	if err != nil {
		c.logger.Printf("failed to initialize target %q: %v", name, err)
		return
	}
	select {
	case <-nctx.Done():
		return
	case c.targetsChan <- c.Targets[name]:
		c.logger.Printf("queuing target %q", name)
	}
	c.logger.Printf("subscribing to target: %q", name)
	go func() {
		err := c.Subscribe(nctx, name)
		if err != nil {
			c.logger.Printf("failed to subscribe: %v", err)
			return
		}
	}()
}

func (c *Collector) DeleteTarget(ctx context.Context, name string) error {
	if c.Targets == nil {
		return nil
	}
	if _, ok := c.Targets[name]; !ok {
		return fmt.Errorf("target %q does not exist", name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.logger.Printf("deleting target %q", name)
	if cfn, ok := c.targetsLocksFn[name]; ok {
		cfn()
	}
	t := c.Targets[name]
	t.Stop()
	delete(c.Targets, name)
	delete(c.targetsConfig, name)
	if c.locker == nil {
		return nil
	}
	if cfn, ok := c.targetsLocksFn[name]; ok {
		cfn()
	}
	return c.locker.Unlock(ctx, c.lockKey(name))
}

func (c *Collector) StopTarget(ctx context.Context, name string) error {
	if c.Targets == nil {
		return nil
	}
	if _, ok := c.Targets[name]; !ok {
		return fmt.Errorf("target %q does not exist", name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.logger.Printf("stopping target %q", name)
	t := c.Targets[name]
	t.Stop()
	delete(c.Targets, name)
	if c.locker == nil {
		return nil
	}
	return c.locker.Unlock(ctx, c.lockKey(name))
}

// AddOutput initializes an output called name, with config cfg if it does not already exist
func (c *Collector) AddOutput(name string, cfg map[string]interface{}) error {
	if c.Outputs == nil {
		c.Outputs = make(map[string]outputs.Output)
	}
	if c.outputsConfig == nil {
		c.outputsConfig = make(map[string]map[string]interface{})
	}
	if _, ok := c.Outputs[name]; ok {
		return fmt.Errorf("output '%s' already exists", name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.outputsConfig[name] = cfg
	return nil
}

func (c *Collector) InitOutput(ctx context.Context, name string, tcs map[string]interface{}) {
	c.m.Lock()
	defer c.m.Unlock()
	if _, ok := c.Outputs[name]; ok {
		return
	}
	if cfg, ok := c.outputsConfig[name]; ok {
		if outType, ok := cfg["type"]; ok {
			c.logger.Printf("starting output type %s", outType)
			if initializer, ok := outputs.Outputs[outType.(string)]; ok {
				out := initializer()
				go func() {
					err := out.Init(ctx, name, cfg,
						outputs.WithLogger(c.logger),
						outputs.WithEventProcessors(c.EventProcessorsConfig, c.logger, tcs),
						outputs.WithRegister(c.reg),
						outputs.WithName(c.Config.Name),
						outputs.WithClusterName(c.Config.ClusterName),
					)
					if err != nil {
						c.logger.Printf("failed to init output type %q: %v", outType, err)
					}
				}()
				c.Outputs[name] = out
			}
		}
	}
}

func (c *Collector) InitOutputs(ctx context.Context) {
	tcs := c.targetsConfigsToMap()
	for name := range c.outputsConfig {
		c.InitOutput(ctx, name, tcs)
	}
}

func (c *Collector) DeleteOutput(name string) error {
	if c.Outputs == nil {
		return nil
	}
	if _, ok := c.Outputs[name]; !ok {
		return fmt.Errorf("output '%s' does not exist", name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	o := c.Outputs[name]
	o.Close()
	return nil
}

// AddSubscriptionConfig adds a subscriptionConfig sc to Collector's map if it does not already exists
func (c *Collector) AddSubscriptionConfig(sc *SubscriptionConfig) error {
	if c.Subscriptions == nil {
		c.Subscriptions = make(map[string]*SubscriptionConfig)
	}
	if _, ok := c.Subscriptions[sc.Name]; ok {
		return fmt.Errorf("subscription '%s' already exists", sc.Name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.Subscriptions[sc.Name] = sc
	return nil
}

func (c *Collector) DeleteSubscription(name string) error {
	if _, ok := c.Subscriptions[name]; !ok {
		return fmt.Errorf("subscription '%s' does not exist", name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	for _, t := range c.Targets {
		if _, ok := t.SubscribeClients[name]; ok {
			t.m.Lock()
			t.subscribeCancelFn[name]()
			delete(t.subscribeCancelFn, name)
			delete(t.SubscribeClients, name)
			delete(t.Subscriptions, name)
			t.m.Unlock()
		}
	}
	delete(c.Subscriptions, name)
	return nil
}

// Subscribe //
func (c *Collector) Subscribe(ctx context.Context, tName string) error {
	c.m.Lock()
	defer c.m.Unlock()
	if t, ok := c.Targets[tName]; ok {
		subscriptionsConfigs := t.Subscriptions
		if len(subscriptionsConfigs) == 0 {
			subscriptionsConfigs = c.Subscriptions
		}
		if len(subscriptionsConfigs) == 0 {
			return fmt.Errorf("target '%s' has no subscriptions defined", tName)
		}
		subRequests := make([]subscriptionRequest, 0)
		for _, sc := range subscriptionsConfigs {
			req, err := sc.CreateSubscribeRequest(tName)
			if err != nil {
				return err
			}
			subRequests = append(subRequests, subscriptionRequest{name: sc.Name, req: req})
		}
		gnmiCtx, cancel := context.WithCancel(ctx)
		t.cfn = cancel
	CRCLIENT:
		if err := t.CreateGNMIClient(gnmiCtx, c.dialOpts...); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				c.logger.Printf("failed to initialize target %q timeout (%s) reached", tName, t.Config.Timeout)
			} else {
				c.logger.Printf("failed to initialize target %q: %v", tName, err)
			}
			c.logger.Printf("retrying target %q in %s", tName, t.Config.RetryTimer)
			time.Sleep(t.Config.RetryTimer)
			goto CRCLIENT

		}
		c.logger.Printf("target '%s' gNMI client created", t.Config.Name)

		for _, sreq := range subRequests {
			c.logger.Printf("sending gNMI SubscribeRequest: subscribe='%+v', mode='%+v', encoding='%+v', to %s",
				sreq.req, sreq.req.GetSubscribe().GetMode(), sreq.req.GetSubscribe().GetEncoding(), t.Config.Name)
			go t.Subscribe(gnmiCtx, sreq.req, sreq.name)
		}
		return nil
	}
	return fmt.Errorf("unknown target name: %s", tName)
}

func (c *Collector) SubscribeOnce(ctx context.Context, tName string) error {
	if t, ok := c.Targets[tName]; ok {
		subscriptionsConfigs := t.Subscriptions
		if len(subscriptionsConfigs) == 0 {
			subscriptionsConfigs = c.Subscriptions
		}
		if len(subscriptionsConfigs) == 0 {
			return fmt.Errorf("target '%s' has no subscriptions defined", tName)
		}
		subRequests := make([]subscriptionRequest, 0)
		for _, sc := range subscriptionsConfigs {
			req, err := sc.CreateSubscribeRequest(tName)
			if err != nil {
				return err
			}
			subRequests = append(subRequests, subscriptionRequest{name: sc.Name, req: req})
		}
		gnmiCtx, cancel := context.WithCancel(ctx)
		t.cfn = cancel
	CRCLIENT:
		if err := t.CreateGNMIClient(gnmiCtx, c.dialOpts...); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				c.logger.Printf("failed to initialize target %q timeout (%s) reached", tName, t.Config.Timeout)
			} else {
				c.logger.Printf("failed to initialize target %q: %v", tName, err)
			}
			c.logger.Printf("retrying target %q in %s", tName, t.Config.RetryTimer)
			time.Sleep(t.Config.RetryTimer)
			goto CRCLIENT

		}
		c.logger.Printf("target '%s' gNMI client created", t.Config.Name)
	OUTER:
		for _, sreq := range subRequests {
			c.logger.Printf("sending gNMI SubscribeRequest: subscribe='%+v', mode='%+v', encoding='%+v', to %s",
				sreq.req, sreq.req.GetSubscribe().GetMode(), sreq.req.GetSubscribe().GetEncoding(), t.Config.Name)
			rspCh, errCh := t.SubscribeOnce(gnmiCtx, sreq.req, sreq.name)
			for {
				select {
				case err := <-errCh:
					if errors.Is(err, io.EOF) {
						c.logger.Printf("target %q, subscription %q closed stream(EOF)", t.Config.Name, sreq.name)
						close(rspCh)
						// next subscription or end
						continue OUTER
					}
					return err
				case rsp := <-rspCh:
					switch rsp.Response.(type) {
					case *gnmi.SubscribeResponse_SyncResponse:
						c.logger.Printf("target %q, subscription %q received sync response", t.Config.Name, sreq.name)
						return nil
					default:
						m := outputs.Meta{"source": t.Config.Name, "format": c.Config.Format, "subscription-name": sreq.name}
						c.Export(ctx, rsp, m, t.Config.Outputs...)
					}
				}
			}
		}
		return nil
	}
	return fmt.Errorf("unknown target name: %s", tName)
}

// Start start the prometheus server as well as a goroutine per target selecting on the response chan, the error chan and the ctx.Done() chan
func (c *Collector) Start(ctx context.Context) {
	if c.httpServer != nil {
		go func() {
			c.logger.Printf("starting prometheus server on %s", c.httpServer.Addr)
			err := c.httpServer.ListenAndServe()
			if err != nil {
				c.logger.Printf("Unable to start prometheus http server: %v", err)
				return
			}
		}()
	}
	defer func() {
		for _, o := range c.Outputs {
			o.Close()
		}
	}()

	for t := range c.targetsChan {
		if c.Config.Debug {
			c.logger.Printf("starting target %+v", t)
		}
		if t == nil {
			continue
		}
		c.m.Lock()
		if _, ok := c.activeTargets[t.Config.Name]; ok {
			if c.Config.Debug {
				c.logger.Printf("target %q listener already active", t.Config.Name)
			}
			c.m.Unlock()
			continue
		}
		c.activeTargets[t.Config.Name] = struct{}{}
		c.m.Unlock()
		c.logger.Printf("starting target %q listener", t.Config.Name)
		go func(t *Target) {
			numOnceSubscriptions := t.numberOfOnceSubscriptions()
			remainingOnceSubscriptions := numOnceSubscriptions
			numSubscriptions := len(t.Subscriptions)
			for {
				select {
				case rsp := <-t.subscribeResponses:
					if c.Config.Debug {
						c.logger.Printf("received gNMI Subscribe Response: %+v", rsp)
					}
					err := t.decodeProtoBytes(rsp.Response)
					if err != nil {
						c.logger.Printf("target %q, failed to decode proto bytes: %v", t.Config.Name, err)
						continue
					}
					m := outputs.Meta{
						"source":              t.Config.Name,
						"format":              c.Config.Format,
						"subscription-name":   rsp.SubscriptionName,
						"subscription-target": rsp.SubscriptionConfig.Target,
					}
					if c.subscriptionMode(rsp.SubscriptionName) == "ONCE" {
						c.Export(ctx, rsp.Response, m, t.Config.Outputs...)
					} else {
						go c.Export(ctx, rsp.Response, m, t.Config.Outputs...)
					}
					if remainingOnceSubscriptions > 0 {
						if c.subscriptionMode(rsp.SubscriptionName) == "ONCE" {
							switch rsp.Response.Response.(type) {
							case *gnmi.SubscribeResponse_SyncResponse:
								remainingOnceSubscriptions--
							}
						}
					}
					if remainingOnceSubscriptions == 0 && numSubscriptions == numOnceSubscriptions {
						c.m.Lock()
						delete(c.activeTargets, t.Config.Name)
						c.m.Unlock()
						return
					}
				case tErr := <-t.errors:
					if errors.Is(tErr.Err, io.EOF) {
						c.logger.Printf("target %q, subscription %s closed stream(EOF)", t.Config.Name, tErr.SubscriptionName)
					} else {
						c.logger.Printf("target %q, subscription %s rcv error: %v", t.Config.Name, tErr.SubscriptionName, tErr.Err)
					}
					if remainingOnceSubscriptions > 0 {
						if c.subscriptionMode(tErr.SubscriptionName) == "ONCE" {
							remainingOnceSubscriptions--
						}
					}
					if remainingOnceSubscriptions == 0 && numSubscriptions == numOnceSubscriptions {
						c.m.Lock()
						delete(c.activeTargets, t.Config.Name)
						c.m.Unlock()
						return
					}
				case <-t.stopChan:
					c.logger.Printf("stopping target %q listener", t.Config.Name)
					c.m.Lock()
					delete(c.activeTargets, t.Config.Name)
					c.m.Unlock()
					return
				case <-ctx.Done():
					c.m.Lock()
					delete(c.activeTargets, t.Config.Name)
					c.m.Unlock()
					return
				}
			}
		}(t)
	}
	for range ctx.Done() {
		return
	}
}

// TargetPoll sends a gnmi.SubscribeRequest_Poll to targetName and returns the response and an error,
// it uses the targetName and the subscriptionName strings to find the gnmi.GNMI_SubscribeClient
func (c *Collector) TargetPoll(targetName, subscriptionName string) (*gnmi.SubscribeResponse, error) {
	if t, ok := c.Targets[targetName]; ok {
		if sub, ok := t.Subscriptions[subscriptionName]; ok {
			if strings.ToUpper(sub.Mode) != "POLL" {
				return nil, fmt.Errorf("subscription '%s' is not a POLL subscription", subscriptionName)
			}
			if subClient, ok := t.SubscribeClients[subscriptionName]; ok {
				err := subClient.Send(&gnmi.SubscribeRequest{
					Request: &gnmi.SubscribeRequest_Poll{
						Poll: &gnmi.Poll{},
					},
				})
				if err != nil {
					return nil, err
				}
				return subClient.Recv()
			}
			return nil, fmt.Errorf("subscribe-client not found '%s'", subscriptionName)
		}
		return nil, fmt.Errorf("unknown subscription name '%s'", subscriptionName)
	}
	return nil, fmt.Errorf("unknown target name '%s'", targetName)
}

// PolledSubscriptionsTargets returns a map of target name to a list of subscription names that have Mode == POLL
func (c *Collector) PolledSubscriptionsTargets() map[string][]string {
	result := make(map[string][]string)
	for tn, target := range c.Targets {
		for _, sub := range target.Subscriptions {
			if strings.ToUpper(sub.Mode) == "POLL" {
				if result[tn] == nil {
					result[tn] = make([]string, 0)
				}
				result[tn] = append(result[tn], sub.Name)
			}
		}
	}
	return result
}

func (c *Collector) subscriptionMode(name string) string {
	if sub, ok := c.Subscriptions[name]; ok {
		return strings.ToUpper(sub.Mode)
	}
	return ""
}

func (c *Collector) Export(ctx context.Context, rsp *gnmi.SubscribeResponse, m outputs.Meta, outs ...string) {
	if rsp == nil {
		return
	}
	wg := new(sync.WaitGroup)
	if len(outs) == 0 {
		wg.Add(len(c.Outputs))
		for _, o := range c.Outputs {
			go func(o outputs.Output) {
				defer wg.Done()
				o.Write(ctx, rsp, m)
			}(o)
		}
		wg.Wait()
		return
	}
	for _, name := range outs {
		if o, ok := c.Outputs[name]; ok {
			wg.Add(1)
			go func(o outputs.Output) {
				defer wg.Done()
				o.Write(ctx, rsp, m)
			}(o)
		}
	}
	wg.Wait()
}

func (c *Collector) Capabilities(ctx context.Context, tName string, ext ...*gnmi_ext.Extension) (*gnmi.CapabilityResponse, error) {
	if _, ok := c.Targets[tName]; !ok {
		err := c.initTarget(tName)
		if err != nil {
			return nil, err
		}
	}
	c.m.Lock()
	defer c.m.Unlock()
	if t, ok := c.Targets[tName]; ok {
		ctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
		defer cancel()
		if t.Client == nil {
			if err := t.CreateGNMIClient(ctx, c.dialOpts...); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return nil, fmt.Errorf("failed to create a gRPC client for target '%s', timeout (%s) reached", t.Config.Name, t.Config.Timeout)
				}
				return nil, fmt.Errorf("failed to create a gRPC client for target '%s' : %v", t.Config.Name, err)
			}
		}
		return t.Capabilities(ctx, ext...)
	}
	return nil, fmt.Errorf("unknown target name: %s", tName)
}

func (c *Collector) Get(ctx context.Context, tName string, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	if _, ok := c.Targets[tName]; !ok {
		err := c.initTarget(tName)
		if err != nil {
			return nil, err
		}
	}
	c.m.Lock()
	defer c.m.Unlock()
	if t, ok := c.Targets[tName]; ok {
		ctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
		defer cancel()
		if t.Client == nil {
			if err := t.CreateGNMIClient(ctx, c.dialOpts...); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return nil, fmt.Errorf("failed to create a gRPC client for target '%s', timeout (%s) reached", t.Config.Name, t.Config.Timeout)
				}
				return nil, fmt.Errorf("failed to create a gRPC client for target '%s' : %v", t.Config.Name, err)
			}
		}
		return t.Get(ctx, req)
	}
	return nil, fmt.Errorf("unknown target name: %s", tName)
}

func (c *Collector) Set(ctx context.Context, tName string, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	if _, ok := c.Targets[tName]; !ok {
		err := c.initTarget(tName)
		if err != nil {
			return nil, err
		}
	}
	c.m.Lock()
	defer c.m.Unlock()
	if t, ok := c.Targets[tName]; ok {
		ctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
		defer cancel()
		if t.Client == nil {
			if err := t.CreateGNMIClient(ctx, c.dialOpts...); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return nil, fmt.Errorf("failed to create a gRPC client for target '%s', timeout (%s) reached", t.Config.Name, t.Config.Timeout)
				}
				return nil, fmt.Errorf("failed to create a gRPC client for target '%s' : %v", t.Config.Name, err)
			}
		}
		return t.Set(ctx, req)
	}
	return nil, fmt.Errorf("unknown target name: %s", tName)
}

func (c *Collector) GetModels(ctx context.Context, tName string) ([]*gnmi.ModelData, error) {
	capRsp, err := c.Capabilities(ctx, tName)
	if err != nil {
		return nil, err
	}
	return capRsp.GetSupportedModels(), nil
}

func (c *Collector) parseProtoFiles(t *Target) error {
	if len(t.Config.ProtoFiles) == 0 {
		t.rootDesc = c.rootDesc
		return nil
	}
	c.logger.Printf("target %q loading proto files...", t.Config.Name)
	descSource, err := grpcurl.DescriptorSourceFromProtoFiles(t.Config.ProtoDirs, t.Config.ProtoFiles...)
	if err != nil {
		c.logger.Printf("failed to load proto files: %v", err)
		return err
	}
	t.rootDesc, err = descSource.FindSymbol("Nokia.SROS.root")
	if err != nil {
		c.logger.Printf("target %q could not get symbol 'Nokia.SROS.root': %v", t.Config.Name, err)
		return err
	}
	c.logger.Printf("target %q loaded proto files", t.Config.Name)
	return nil
}

func (c *Collector) targetsConfigsToMap() map[string]interface{} {
	tcs := make(map[string]interface{})
	var err error
	for n, tc := range c.targetsConfig {
		var itc interface{}
		err = mapstructure.Decode(tc, &itc)
		if err != nil {
			c.logger.Printf("failed to decode target %q config: %v", n, err)
			continue
		}
		c.logger.Printf("%T | %v", itc, itc)
		tcs[n] = itc
	}
	return tcs
}
