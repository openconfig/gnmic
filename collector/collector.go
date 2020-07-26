package collector

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/proto/gnmi"
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
}

// Collector //
type Collector struct {
	Config        *Config
	Subscriptions map[string]*SubscriptionConfig
	Outputs       map[string][]outputs.Output
	DialOpts      []grpc.DialOption
	//
	m          *sync.Mutex
	Targets    map[string]*Target
	Logger     *log.Logger
	httpServer *http.Server

	ctx      context.Context
	cancelFn context.CancelFunc
}

// NewCollector //
func NewCollector(ctx context.Context,
	config *Config,
	targetConfigs map[string]*TargetConfig,
	subscriptions map[string]*SubscriptionConfig,
	outputs map[string][]outputs.Output,
	dialOpts []grpc.DialOption,
	logger *log.Logger,
) *Collector {
	nctx, cancel := context.WithCancel(ctx)
	grpcMetrics := grpc_prometheus.NewClientMetrics()
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	grpcMetrics.EnableClientHandlingTimeHistogram()
	reg.MustRegister(grpcMetrics)
	handler := http.NewServeMux()
	handler.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	httpServer := &http.Server{
		Handler: handler,
		Addr:    config.PrometheusAddress,
	}
	dialOpts = append(dialOpts, grpc.WithStreamInterceptor(grpcMetrics.StreamClientInterceptor()))
	if config.TargetReceiveBuffer == 0 {
		config.TargetReceiveBuffer = defaultTargetReceivebuffer
	}
	c := &Collector{
		Config:        config,
		Subscriptions: subscriptions,
		Outputs:       outputs,
		DialOpts:      dialOpts,
		m:             new(sync.Mutex),
		Targets:       make(map[string]*Target),
		Logger:        logger,
		httpServer:    httpServer,
		ctx:           nctx,
		cancelFn:      cancel,
	}

	for _, tc := range targetConfigs {
		c.InitTarget(tc)
	}
	return c
}

// InitTarget initializes a target based on *TargetConfig
func (c *Collector) InitTarget(tc *TargetConfig) {
	if tc.BufferSize == 0 {
		tc.BufferSize = c.Config.TargetReceiveBuffer
	}
	t := NewTarget(tc)
	//
	t.Subscriptions = make([]*SubscriptionConfig, 0, len(tc.Subscriptions))
	for _, subName := range tc.Subscriptions {
		if sub, ok := c.Subscriptions[subName]; ok {
			t.Subscriptions = append(t.Subscriptions, sub)
		}
	}
	if len(t.Subscriptions) == 0 {
		t.Subscriptions = make([]*SubscriptionConfig, 0, len(c.Subscriptions))
		for _, sub := range c.Subscriptions {
			t.Subscriptions = append(t.Subscriptions, sub)
		}
	}
	//
	t.Outputs = make([]outputs.Output, 0, len(tc.Outputs))
	for _, outName := range tc.Outputs {
		if outs, ok := c.Outputs[outName]; ok {
			t.Outputs = append(t.Outputs, outs...)
		}
	}
	if len(t.Outputs) == 0 {
		t.Outputs = make([]outputs.Output, 0, len(c.Outputs))
		for _, o := range c.Outputs {
			t.Outputs = append(t.Outputs, o...)
		}
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.Targets[t.Config.Name] = t
}

// Subscribe //
func (c *Collector) Subscribe(tName string) error {
	if t, ok := c.Targets[tName]; ok {
		if err := t.CreateGNMIClient(c.ctx, c.DialOpts...); err != nil {
			return err
		}
		c.Logger.Printf("target '%s' gNMI client created", t.Config.Name)
		for _, sc := range t.Subscriptions {
			req, err := sc.CreateSubscribeRequest()
			if err != nil {
				return err
			}
			c.Logger.Printf("sending gNMI SubscribeRequest: subscribe='%+v', mode='%+v', encoding='%+v', to %s",
				req, req.GetSubscribe().GetMode(), req.GetSubscribe().GetEncoding(), t.Config.Name)
			go t.Subscribe(c.ctx, req, sc.Name)
		}
		return nil
	}
	return fmt.Errorf("unknown target name: %s", tName)
}

// Start start the prometheus server as well as a goroutine per target selecting on the response chan, the error chan and the ctx.Done() chan
func (c *Collector) Start() {
	go func() {
		if err := c.httpServer.ListenAndServe(); err != nil {
			c.Logger.Printf("Unable to start prometheus http server: %v", err)
			return
		}
	}()
	defer func() {
		for _, outputs := range c.Outputs {
			for _, o := range outputs {
				o.Close()
			}
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
				case rsp := <-t.SubscribeResponses:
					if c.Config.Debug {
						c.Logger.Printf("received gNMI Subscribe Response: %+v", rsp)
					}
					if c.subscriptionMode(rsp.SubscriptionName) == "ONCE" {
						t.Export(rsp.Response, outputs.Meta{"source": t.Config.Name, "format": c.Config.Format, "subscription-name": rsp.SubscriptionName})
					} else {
						go t.Export(rsp.Response, outputs.Meta{"source": t.Config.Name, "format": c.Config.Format, "subscription-name": rsp.SubscriptionName})
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
				case err := <-t.Errors:
					if err == io.EOF {
						c.Logger.Printf("target '%s' closed stream(EOF)", t.Config.Name)
						return
					}
					c.Logger.Printf("target '%s' error: %v", t.Config.Name, err)
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
