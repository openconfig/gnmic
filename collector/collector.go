package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

type Config struct {
	PrometheusAddress string
	Debug             bool
}

// Collector //
type Collector struct {
	Config        *Config
	Subscriptions map[string]*SubscriptionConfig
	Outputs       map[string]outputs.Output
	DialOpts      []grpc.DialOption
	//
	m          *sync.Mutex
	Targets    map[string]*Target
	Logger     *log.Logger
	httpServer *http.Server
	ctx        context.Context
	cancelFn   context.CancelFunc
}

// NewCollector //
func NewCollector(ctx context.Context,
	config *Config,
	targetConfigs map[string]*TargetConfig,
	subscriptions map[string]*SubscriptionConfig,
	outputs map[string]outputs.Output,
	dialOpts []grpc.DialOption,
	logger *log.Logger,
) *Collector {
	nctx, cancel := context.WithCancel(ctx)
	grpcMetrics := grpc_prometheus.NewClientMetrics()
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	grpcMetrics.EnableClientHandlingTimeHistogram()
	reg.MustRegister(grpcMetrics)
	httpServer := &http.Server{
		Handler: promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		Addr:    config.PrometheusAddress,
	}
	c := &Collector{
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
	wg := new(sync.WaitGroup)
	wg.Add(len(targetConfigs))
	for _, tc := range targetConfigs {
		go func(tc *TargetConfig) {
			defer wg.Done()
			err := c.InitTarget(tc)
			if err != nil {
				c.Logger.Printf("failed to initialize target '%s': %v", tc.Name, err)
				return
			}
			c.Logger.Printf("target '%s' initialized", tc.Name)
		}(tc)
	}
	wg.Wait()
	return c
}

// InitTarget initializes a target based on *TargetConfig
func (c *Collector) InitTarget(tc *TargetConfig) error {
	t, err := NewTarget(tc)
	if err != nil {
		return err
	}
	t.Subscriptions = make([]*SubscriptionConfig, 0, len(tc.Subscriptions))
	for _, subName := range tc.Subscriptions {
		if sub, ok := c.Subscriptions[subName]; ok {
			t.Subscriptions = append(t.Subscriptions, sub)
		}
	}
	t.Outputs = make([]outputs.Output, 0, len(tc.Outputs))
	for _, outName := range tc.Outputs {
		if o, ok := c.Outputs[outName]; ok {
			t.Outputs = append(t.Outputs, o)
		}
	}
	err = t.CreateGNMIClient(c.ctx, c.DialOpts...)
	if err != nil {
		return err
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.Targets[t.Config.Name] = t
	return nil
}

// Subscribe //
func (c *Collector) Subscribe(tName string) error {
	if t, ok := c.Targets[tName]; ok {
		for _, sc := range t.Subscriptions {
			req, err := sc.CreateSubscribeRequest()
			if err != nil {
				return err
			}
			go t.Subscribe(c.ctx, req, sc.Name)
		}
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
	wg := new(sync.WaitGroup)
	for _, t := range c.Targets {
		go func(t *Target) {
			defer wg.Done()
			for {
				select {
				case rsp := <-t.SubscribeResponses:
					m := make(map[string]interface{})
					m["subscription-name"] = rsp.SubscriptionName
					b, err := c.FormatMsg(m, rsp.Response)
					if err != nil {
						c.Logger.Printf("failed formatting msg from target '%s': %v", t.Config.Name, err)
					}
					go t.Export(b, outputs.Meta{"source": t.Config.Name})
				case err := <-t.Errors:
					c.Logger.Printf("target '%s' error: %v", t.Config.Name, err)
				case <-c.ctx.Done():
					return
				}
			}
		}(t)
	}
	wg.Wait()
}

func (c *Collector) FormatMsg(meta map[string]interface{}, rsp *gnmi.SubscribeResponse) ([]byte, error) {
	switch rsp := rsp.Response.(type) {
	case *gnmi.SubscribeResponse_Update:
		msg := new(msg)
		msg.Timestamp = rsp.Update.Timestamp
		t := time.Unix(0, rsp.Update.Timestamp)
		msg.Time = &t
		if meta == nil {
			meta = make(map[string]interface{})
		}
		msg.Prefix = gnmiPathToXPath(rsp.Update.Prefix)
		var ok bool
		if _, ok = meta["source"]; ok {
			msg.Source = fmt.Sprintf("%s", meta["source"])
		}
		if _, ok = meta["system-name"]; ok {
			msg.SystemName = fmt.Sprintf("%s", meta["system-name"])
		}
		if _, ok = meta["subscription-name"]; ok {
			msg.SubscriptionName = fmt.Sprintf("%s", meta["subscription-name"])
		}
		for i, upd := range rsp.Update.Update {
			pathElems := make([]string, 0, len(upd.Path.Elem))
			for _, pElem := range upd.Path.Elem {
				pathElems = append(pathElems, pElem.GetName())
			}
			value, err := getValue(upd.Val)
			if err != nil {
				c.Logger.Println(err)
			}
			msg.Updates = append(msg.Updates,
				&update{
					Path:   gnmiPathToXPath(upd.Path),
					Values: make(map[string]interface{}),
				})
			msg.Updates[i].Values[strings.Join(pathElems, "/")] = value
		}
		for _, del := range rsp.Update.Delete {
			msg.Deletes = append(msg.Deletes, gnmiPathToXPath(del))
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, nil
}
