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

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

const (
	defaultTargetReceivebuffer = 1000
)

// Config is the collector config
type Config struct {
	PrometheusAddress   string
	Debug               bool
	Format              string
	TargetReceiveBuffer uint
	RetryTimer          time.Duration
}

// Collector //
type Collector struct {
	Config   *Config
	dialOpts []grpc.DialOption
	//
	m             *sync.Mutex
	Subscriptions map[string]*SubscriptionConfig
	Outputs       map[string]outputs.Output
	Targets       map[string]*Target
	logger        *log.Logger
	httpServer    *http.Server
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

func WithOutputs(ctx context.Context, outs map[string]map[string]interface{}, logger *log.Logger) CollectorOption {
	return func(c *Collector) {
		for outputName, outputCfg := range outs {
			c.AddOutput(ctx, outputName, outputCfg, logger)
		}
	}
}

func WithDialOptions(dialOptions []grpc.DialOption) CollectorOption {
	return func(c *Collector) {
		c.dialOpts = dialOptions
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

	c := &Collector{
		Config:     config,
		m:          new(sync.Mutex),
		Targets:    make(map[string]*Target),
		Outputs:    make(map[string]outputs.Output),
		httpServer: httpServer,
	}
	for _, op := range opts {
		op(c)
	}
	if config.Debug {
		c.logger.Printf("starting collector with cfg=%+v", config)
	}
	if config.PrometheusAddress != "" {
		grpcMetrics := grpc_prometheus.NewClientMetrics()
		reg := prometheus.NewRegistry()
		reg.MustRegister(prometheus.NewGoCollector())
		reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
		grpcMetrics.EnableClientHandlingTimeHistogram()
		reg.MustRegister(grpcMetrics)
		handler := http.NewServeMux()
		handler.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
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
	if _, ok := c.Targets[tc.Name]; ok {
		return fmt.Errorf("target '%s' already exists", tc.Name)
	}
	if tc.BufferSize == 0 {
		tc.BufferSize = c.Config.TargetReceiveBuffer
	}
	if tc.RetryTimer == 0 {
		tc.RetryTimer = c.Config.RetryTimer
	}
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
	c.m.Lock()
	defer c.m.Unlock()
	c.Targets[t.Config.Name] = t
	return nil
}

// AddOutput initializes an output called name, with config cfg if it does not already exist
func (c *Collector) AddOutput(ctx context.Context, name string, cfg map[string]interface{}, logger *log.Logger) error {
	if _, ok := c.Outputs[name]; ok {
		return fmt.Errorf("output '%s' already exists", name)
	}
	if outType, ok := cfg["type"]; ok {
		if initializer, ok := outputs.Outputs[outType.(string)]; ok {
			out := initializer()
			go out.Init(ctx, cfg, logger)
			c.m.Lock()
			defer c.m.Unlock()
			c.Outputs[name] = out
		}
	}
	return nil
}

// AddSubscriptionConfig adds a subscriptionConfig sc to Collector's map if it does not already exists
func (c *Collector) AddSubscriptionConfig(sc *SubscriptionConfig) error {
	if _, ok := c.Subscriptions[sc.Name]; ok {
		return fmt.Errorf("subscription '%s' already exists", sc.Name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.Subscriptions[sc.Name] = sc
	return nil
}

// Subscribe //
func (c *Collector) Subscribe(ctx context.Context, tName string) error {
	if t, ok := c.Targets[tName]; ok {
		if err := t.CreateGNMIClient(ctx, c.dialOpts...); err != nil {
			return err
		}
		c.logger.Printf("target '%s' gNMI client created", t.Config.Name)
		for _, sc := range t.Subscriptions {
			req, err := sc.CreateSubscribeRequest()
			if err != nil {
				return err
			}
			c.logger.Printf("sending gNMI SubscribeRequest: subscribe='%+v', mode='%+v', encoding='%+v', to %s",
				req, req.GetSubscribe().GetMode(), req.GetSubscribe().GetEncoding(), t.Config.Name)
			go t.Subscribe(ctx, req, sc.Name)
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
	wg := new(sync.WaitGroup)
	wg.Add(len(c.Targets))
	for _, t := range c.Targets {
		go func(t *Target) {
			defer wg.Done()
			numOnceSubscriptions := t.numberOfOnceSubscriptions()
			remainingOnceSubscriptions := numOnceSubscriptions
			numSubscriptions := len(t.Subscriptions)
			for {
				select {
				case rsp := <-t.subscribeResponses:
					if c.Config.Debug {
						c.logger.Printf("received gNMI Subscribe Response: %+v", rsp)
					}
					m := outputs.Meta{"source": t.Config.Name, "format": c.Config.Format, "subscription-name": rsp.SubscriptionName}
					if c.subscriptionMode(rsp.SubscriptionName) == "ONCE" {
						c.Export(ctx, rsp.Response, m, t.Config.Outputs...)
					} else {
						//go t.Export(ctx, rsp.Response, outputs.Meta{"source": t.Config.Name, "format": c.Config.Format, "subscription-name": rsp.SubscriptionName})
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
						return
					}
				case tErr := <-t.errors:
					if errors.Is(tErr.Err, io.EOF) {
						c.logger.Printf("target '%s', subscription %s closed stream(EOF)", t.Config.Name, tErr.SubscriptionName)
					} else {
						c.logger.Printf("target '%s', subscription %s rcv error: %v", t.Config.Name, tErr.SubscriptionName, tErr.Err)
					}
					if remainingOnceSubscriptions > 0 {
						if c.subscriptionMode(tErr.SubscriptionName) == "ONCE" {
							remainingOnceSubscriptions--
						}
					}
					if remainingOnceSubscriptions == 0 && numSubscriptions == numOnceSubscriptions {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}(t)
	}
	wg.Wait()
}

// TargetPoll sends a gnmi.SubscribeRequest_Poll to targetName and returns the response and an error,
// it uses the targetName and the subscriptionName strings to find the gnmi.GNMI_SubscribeClient
func (c *Collector) TargetPoll(targetName, subscriptionName string) (*gnmi.SubscribeResponse, error) {
	if sub, ok := c.Subscriptions[subscriptionName]; ok {
		if strings.ToUpper(sub.Mode) != "POLL" {
			return nil, fmt.Errorf("subscription '%s' is not a POLL subscription", subscriptionName)
		}
		if t, ok := c.Targets[targetName]; ok {
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
		}
		return nil, fmt.Errorf("unknown target name '%s'", targetName)
	}
	return nil, fmt.Errorf("unknown subscription name '%s'", subscriptionName)
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
