// © 2025 NVIDIA Corporation
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of NVIDIA's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package otlp_output

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmi/proto/gnmi"
	gutils "github.com/openconfig/gnmic/pkg/utils"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/zestor-dev/zestor/store"
)

const (
	outputType        = "otlp"
	defaultTimeout    = 10 * time.Second
	defaultBatchSize  = 1000
	defaultNumWorkers = 1
	defaultMaxRetries = 3
	defaultProtocol   = "grpc"
	loggingPrefix     = "[otlp_output:%s] "
)

func init() {
	outputs.Register(outputType, func() outputs.Output {
		return &otlpOutput{}
	})
}

// otlpOutput implements the Output interface for OTLP metrics export
type otlpOutput struct {
	outputs.BaseOutput

	cfg       *atomic.Pointer[config]
	dynCfg    *atomic.Pointer[dynConfig]
	grpcState *atomic.Pointer[grpcClientState]
	eventCh   *atomic.Pointer[chan *formatters.EventMsg]

	logger   *log.Logger
	rootCtx  context.Context
	cancelFn context.CancelFunc
	wg       *sync.WaitGroup

	// Metrics
	reg *prometheus.Registry
	// store
	store store.Store[any]
}

type dynConfig struct {
	evps []formatters.EventProcessor
}

type grpcClientState struct {
	conn   *grpc.ClientConn
	client metricsv1.MetricsServiceClient
}

// config holds the OTLP output configuration
type config struct {
	// name of the output
	Name string `mapstructure:"name,omitempty"`
	// endpoint of the OTLP collector
	Endpoint string `mapstructure:"endpoint,omitempty"`
	// "grpc" or "http"
	// defaults to "grpc"
	Protocol string `mapstructure:"protocol,omitempty"`
	// RPC timeout
	Timeout time.Duration `mapstructure:"timeout,omitempty"`
	// TLS configuration
	TLS *types.TLSConfig `mapstructure:"tls,omitempty"`

	// Batching
	BatchSize  int           `mapstructure:"batch-size,omitempty"`
	Interval   time.Duration `mapstructure:"interval,omitempty"`
	BufferSize int           `mapstructure:"buffer-size,omitempty"`

	// Retry
	MaxRetries int `mapstructure:"max-retries,omitempty"`

	// Metric naming
	// string, to be used as the metric namespace
	MetricPrefix string `mapstructure:"metric-prefix,omitempty"`
	// boolean, if true the subscription name will be prepended to the metric name after the prefix.
	AppendSubscriptionName bool `mapstructure:"append-subscription-name,omitempty"`
	// boolean, if true, string type values are exported as gauge metrics with value=1
	// and the string stored as an attribute named "value".
	// if false, string values are dropped.
	StringsAsAttributes bool `mapstructure:"strings-as-attributes,omitempty"`

	// Tags whose values are placed as OTLP Resource attributes and excluded
	// from data point attributes.
	// Set to an empty list to put all tags on data points (useful for Prometheus compatibility).
	ResourceTagKeys []string `mapstructure:"resource-tag-keys,omitempty"`

	// Regex patterns matched against the value key to classify a metric as a
	// monotonic cumulative counter (Sum). Unmatched metrics become Gauges.
	// If empty, all metrics are exported as Gauges.
	CounterPatterns []string `mapstructure:"counter-patterns,omitempty"`

	// Resource attributes
	ResourceAttributes map[string]string `mapstructure:"resource-attributes,omitempty"`

	// Precomputed lookup set for ResourceTagKeys (not from config file).
	resourceTagSet map[string]bool
	// Compiled regexes from CounterPatterns.
	counterRegexes []*regexp.Regexp

	// Performance
	NumWorkers int `mapstructure:"num-workers,omitempty"`

	// Debugging
	Debug         bool `mapstructure:"debug,omitempty"`
	EnableMetrics bool `mapstructure:"enable-metrics,omitempty"`

	// Event processors
	EventProcessors []string `mapstructure:"event-processors,omitempty"`
}

func (o *otlpOutput) initFields() {
	o.cfg = new(atomic.Pointer[config])
	o.dynCfg = new(atomic.Pointer[dynConfig])
	o.grpcState = new(atomic.Pointer[grpcClientState])
	o.eventCh = new(atomic.Pointer[chan *formatters.EventMsg])
	o.wg = new(sync.WaitGroup)
	o.logger = log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags)
}

func (o *otlpOutput) String() string {
	cfg := o.cfg.Load()
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

// Init initializes the OTLP output
func (o *otlpOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	o.initFields()

	ncfg := new(config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}
	if ncfg.Name == "" {
		ncfg.Name = name
	}
	o.logger.SetPrefix(fmt.Sprintf(loggingPrefix, ncfg.Name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}
	o.store = options.Store

	// Set defaults
	if options.Name != "" {
		ncfg.Name = options.Name
	}
	o.setDefaultsFor(ncfg)

	// Apply logger
	if options.Logger != nil {
		o.logger.SetOutput(options.Logger.Writer())
		o.logger.SetFlags(options.Logger.Flags())
	}

	o.cfg.Store(ncfg)

	// Initialize registry
	o.reg = options.Registry
	err = o.registerMetrics()
	if err != nil {
		return err
	}

	// Initialize event processors
	dc := new(dynConfig)
	dc.evps, err = o.buildEventProcessors(ncfg)
	if err != nil {
		return err
	}
	o.dynCfg.Store(dc)

	// Initialize transport
	switch ncfg.Protocol {
	case "grpc":
		gs, err := o.initGRPCFor(ncfg)
		if err != nil {
			return fmt.Errorf("failed to initialize gRPC transport: %w", err)
		}
		o.grpcState.Store(gs)
	case "http":
		return fmt.Errorf("HTTP transport not yet implemented")
	default:
		return fmt.Errorf("unsupported protocol '%s': must be 'grpc' or 'http'", ncfg.Protocol)
	}

	// Initialize worker channels
	eventCh := make(chan *formatters.EventMsg, ncfg.BufferSize)
	o.eventCh.Store(&eventCh)

	// Start workers
	o.rootCtx = ctx
	var wctx context.Context
	wctx, o.cancelFn = context.WithCancel(o.rootCtx)
	o.wg.Add(ncfg.NumWorkers)
	for i := 0; i < ncfg.NumWorkers; i++ {
		go o.worker(wctx, i)
	}

	o.logger.Printf("initialized OTLP output: endpoint=%s, protocol=%s, batch-size=%d, workers=%d",
		ncfg.Endpoint, ncfg.Protocol, ncfg.BatchSize, ncfg.NumWorkers)

	return nil
}

func (o *otlpOutput) Update(ctx context.Context, cfg map[string]any) error {
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}

	o.setDefaultsFor(newCfg)
	if err := o.validateConfig(newCfg); err != nil {
		return err
	}

	currCfg := o.cfg.Load()

	swapChannel := channelNeedsSwap(currCfg, newCfg)
	restartWorkers := needsWorkerRestart(currCfg, newCfg)
	rebuildGRPC := needsGRPCRebuild(currCfg, newCfg)
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0

	dc := new(dynConfig)
	prevDC := o.dynCfg.Load()
	if rebuildProcessors {
		dc.evps, err = o.buildEventProcessors(newCfg)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}
	o.dynCfg.Store(dc)

	if rebuildGRPC {
		gs, err := o.initGRPCFor(newCfg)
		if err != nil {
			return fmt.Errorf("failed to rebuild gRPC transport: %w", err)
		}
		oldState := o.grpcState.Swap(gs)
		if oldState != nil && oldState.conn != nil {
			oldState.conn.Close()
		}
	}

	o.cfg.Store(newCfg)

	if swapChannel || restartWorkers {
		var newChan chan *formatters.EventMsg
		if swapChannel {
			newChan = make(chan *formatters.EventMsg, newCfg.BufferSize)
		} else {
			newChan = *o.eventCh.Load()
		}

		runCtx, cancel := context.WithCancel(o.rootCtx)
		newWG := new(sync.WaitGroup)

		oldCancel := o.cancelFn
		oldWG := o.wg
		oldEventCh := *o.eventCh.Load()

		o.cancelFn = cancel
		o.wg = newWG
		o.eventCh.Store(&newChan)

		o.wg.Add(newCfg.NumWorkers)
		for i := 0; i < newCfg.NumWorkers; i++ {
			go o.worker(runCtx, i)
		}

		if oldCancel != nil {
			oldCancel()
		}
		if oldWG != nil {
			oldWG.Wait()
		}

		if swapChannel {
		OUTER_LOOP:
			for {
				select {
				case ev, ok := <-oldEventCh:
					if !ok {
						break OUTER_LOOP
					}
					select {
					case newChan <- ev:
					default:
					}
				default:
					break OUTER_LOOP
				}
			}
		}
	}

	o.logger.Printf("updated OTLP output: %s", o.String())
	return nil
}

func (o *otlpOutput) Validate(cfg map[string]any) error {
	ncfg := new(config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}
	o.setDefaultsFor(ncfg)
	return o.validateConfig(ncfg)
}

func (o *otlpOutput) UpdateProcessor(name string, pcfg map[string]any) error {
	cfg := o.cfg.Load()
	dc := o.dynCfg.Load()

	newEvps, changed, err := outputs.UpdateProcessorInSlice(
		o.logger,
		o.store,
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
		o.dynCfg.Store(&newDC)
		o.logger.Printf("updated event processor %s", name)
	}
	return nil
}

// Write handles incoming gNMI messages
func (o *otlpOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	cfg := o.cfg.Load()
	dc := o.dynCfg.Load()
	if dc == nil {
		return
	}

	// Type assert to gNMI SubscribeResponse
	subsResp, ok := rsp.(*gnmi.SubscribeResponse)
	if !ok {
		if cfg.Debug {
			o.logger.Printf("received non-SubscribeResponse message, ignoring")
		}
		return
	}

	// Convert gNMI response to EventMsg format
	subscriptionName := meta["subscription-name"]
	if subscriptionName == "" {
		subscriptionName = "default"
	}

	events, err := formatters.ResponseToEventMsgs(subscriptionName, subsResp, meta, dc.evps...)
	if err != nil {
		if cfg.Debug {
			o.logger.Printf("failed to convert response to events: %v", err)
		}
		return
	}

	// Send events to worker channel
	eventCh := *o.eventCh.Load()
	for _, event := range events {
		select {
		case eventCh <- event:
		case <-ctx.Done():
			return
		default:
			if cfg.Debug {
				o.logger.Printf("event channel full, dropping event")
			}
			if cfg.EnableMetrics {
				otlpNumberOfFailedEvents.WithLabelValues(cfg.Name, "channel_full").Inc()
			}
		}
	}
}

// WriteEvent handles individual EventMsg
func (o *otlpOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	if ev == nil {
		return
	}

	cfg := o.cfg.Load()
	eventCh := *o.eventCh.Load()

	select {
	case eventCh <- ev:
	case <-ctx.Done():
		return
	default:
		if cfg.Debug {
			o.logger.Printf("event channel full, dropping event")
		}
		if cfg.EnableMetrics {
			otlpNumberOfFailedEvents.WithLabelValues(cfg.Name, "channel_full").Inc()
		}
	}
}

// Close closes the OTLP output
func (o *otlpOutput) Close() error {
	if o.cancelFn != nil {
		o.cancelFn()
	}

	// Close event channel
	eventCh := o.eventCh.Load()
	if eventCh != nil {
		close(*eventCh)
	}

	// Wait for workers to finish
	o.wg.Wait()

	// Close gRPC connection
	gs := o.grpcState.Load()
	if gs != nil && gs.conn != nil {
		return gs.conn.Close()
	}

	return nil
}

// worker processes events in batches
func (o *otlpOutput) worker(ctx context.Context, id int) {
	defer o.wg.Done()

	cfg := o.cfg.Load()
	if cfg.Debug {
		o.logger.Printf("worker %d started", id)
	}

	batch := make([]*formatters.EventMsg, 0, cfg.BatchSize)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	eventCh := *o.eventCh.Load()

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				flushCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
				defer cancel()
				o.sendBatch(flushCtx, batch)
			}
			if cfg.Debug {
				o.logger.Printf("worker %d stopped", id)
			}
			return

		case event, ok := <-eventCh:
			if !ok {
				if len(batch) > 0 {
					flushCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
					defer cancel()
					o.sendBatch(flushCtx, batch)
				}
				return
			}

			batch = append(batch, event)
			if len(batch) >= cfg.BatchSize {
				o.sendBatch(ctx, batch)
				batch = make([]*formatters.EventMsg, 0, cfg.BatchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				o.sendBatch(ctx, batch)
				batch = make([]*formatters.EventMsg, 0, cfg.BatchSize)
			}
		}
	}
}

func (o *otlpOutput) sendBatch(ctx context.Context, events []*formatters.EventMsg) {
	if len(events) == 0 {
		return
	}

	cfg := o.cfg.Load()
	start := time.Now()

	req := o.convertToOTLP(events)

	var err error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		err = o.sendGRPC(ctx, req)

		if err == nil {
			if cfg.Debug {
				o.logger.Printf("successfully sent %d events (attempt %d)", len(events), attempt+1)
			}
			if cfg.EnableMetrics {
				otlpNumberOfSentEvents.WithLabelValues(cfg.Name).Add(float64(len(events)))
				otlpSendDuration.WithLabelValues(cfg.Name).Observe(time.Since(start).Seconds())
			}
			return
		}

		if attempt < cfg.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}

	o.logger.Printf("failed to send batch after %d retries: %v", cfg.MaxRetries, err)
	if cfg.EnableMetrics {
		otlpNumberOfFailedEvents.WithLabelValues(cfg.Name, "send_failed").Add(float64(len(events)))
	}
}

func (o *otlpOutput) setDefaultsFor(c *config) {
	if c.Timeout == 0 {
		c.Timeout = defaultTimeout
	}
	if c.BatchSize == 0 {
		c.BatchSize = defaultBatchSize
	}
	if c.NumWorkers == 0 {
		c.NumWorkers = defaultNumWorkers
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = defaultMaxRetries
	}
	if c.Protocol == "" {
		c.Protocol = defaultProtocol
	}
	if c.Name == "" {
		c.Name = "gnmic-otlp-" + uuid.New().String()
	}
	if c.Interval == 0 {
		c.Interval = 5 * time.Second
	}
	if c.BufferSize == 0 {
		c.BufferSize = c.BatchSize * 2
	}
	c.resourceTagSet = make(map[string]bool, len(c.ResourceTagKeys))
	for _, k := range c.ResourceTagKeys {
		c.resourceTagSet[k] = true
	}
}

func (o *otlpOutput) validateConfig(c *config) error {
	if c.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	c.counterRegexes = make([]*regexp.Regexp, 0, len(c.CounterPatterns))
	for _, p := range c.CounterPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("invalid counter-pattern %q: %w", p, err)
		}
		c.counterRegexes = append(c.counterRegexes, re)
	}
	return nil
}

func (o *otlpOutput) buildEventProcessors(cfg *config) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(o.store)
	if err != nil {
		return nil, err
	}
	return formatters.MakeEventProcessors(o.logger, cfg.EventProcessors, ps, tcs, acts)
}

func (o *otlpOutput) initGRPCFor(cfg *config) (*grpcClientState, error) {
	var opts []grpc.DialOption

	if cfg.TLS != nil {
		tlsConfig, err := o.createTLSConfigFor(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(cfg.Endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP client: %w", err)
	}

	o.logger.Printf("initialized OTLP gRPC client for endpoint: %s", cfg.Endpoint)
	return &grpcClientState{
		conn:   conn,
		client: metricsv1.NewMetricsServiceClient(conn),
	}, nil
}

func (o *otlpOutput) createTLSConfigFor(cfg *config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.TLS.SkipVerify,
	}

	if cfg.TLS.CaFile != "" || cfg.TLS.CertFile != "" {
		return utils.NewTLSConfig(
			cfg.TLS.CaFile,
			cfg.TLS.CertFile,
			cfg.TLS.KeyFile,
			"",
			cfg.TLS.SkipVerify,
			false,
		)
	}

	return tlsConfig, nil
}

func (o *otlpOutput) registerMetrics() error {
	cfg := o.cfg.Load()
	if !cfg.EnableMetrics {
		return nil
	}

	if o.reg == nil {
		return nil
	}

	if err := o.reg.Register(otlpNumberOfSentEvents); err != nil {
		return err
	}
	if err := o.reg.Register(otlpNumberOfFailedEvents); err != nil {
		return err
	}
	if err := o.reg.Register(otlpSendDuration); err != nil {
		return err
	}
	if err := o.reg.Register(otlpRejectedDataPoints); err != nil {
		return err
	}

	return nil
}

// Helper functions for detecting config changes

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
		old.BatchSize != nw.BatchSize ||
		old.Interval != nw.Interval
}

func needsGRPCRebuild(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.Endpoint != nw.Endpoint ||
		old.Protocol != nw.Protocol ||
		!old.TLS.Equal(nw.TLS)
}
