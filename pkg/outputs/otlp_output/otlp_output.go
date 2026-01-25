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
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmi/proto/gnmi"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
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
		return &otlpOutput{
			cfg:    &config{},
			wg:     new(sync.WaitGroup),
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// otlpOutput implements the Output interface for OTLP metrics export
type otlpOutput struct {
	outputs.BaseOutput
	cfg      *config
	logger   *log.Logger
	ctx      context.Context
	cancelFn context.CancelFunc
	mo       *formatters.MarshalOptions
	evps     []formatters.EventProcessor

	// gRPC client
	grpcConn   *grpc.ClientConn
	grpcClient metricsv1.MetricsServiceClient

	// HTTP client (for future HTTP support)
	// httpClient *http.Client

	// Worker management
	eventCh chan *formatters.EventMsg
	wg      *sync.WaitGroup

	// Metrics
	reg *prometheus.Registry
}

// config holds the OTLP output configuration
type config struct {
	Name     string           `mapstructure:"name,omitempty"`
	Endpoint string           `mapstructure:"endpoint,omitempty"`
	Protocol string           `mapstructure:"protocol,omitempty"` // "grpc" or "http"
	Timeout  time.Duration    `mapstructure:"timeout,omitempty"`
	TLS      *types.TLSConfig `mapstructure:"tls,omitempty"`

	// Batching
	BatchSize int           `mapstructure:"batch-size,omitempty"`
	Interval  time.Duration `mapstructure:"interval,omitempty"`

	// Retry
	MaxRetries int `mapstructure:"max-retries,omitempty"`

	// Metric naming
	MetricPrefix           string `mapstructure:"metric-prefix,omitempty"`
	AppendSubscriptionName bool   `mapstructure:"append-subscription-name,omitempty"`
	StringsAsAttributes    bool   `mapstructure:"strings-as-attributes,omitempty"`

	// Event tags behavior
	AddEventTagsAsAttributes bool `mapstructure:"add-event-tags-as-attributes,omitempty"`

	// Resource attributes
	ResourceAttributes map[string]string `mapstructure:"resource-attributes,omitempty"`

	// Performance
	NumWorkers int `mapstructure:"num-workers,omitempty"`
	BufferSize int `mapstructure:"buffer-size,omitempty"`

	// Debugging
	Debug         bool `mapstructure:"debug,omitempty"`
	EnableMetrics bool `mapstructure:"enable-metrics,omitempty"`

	// Event processors
	EventProcessors []string `mapstructure:"event-processors,omitempty"`
}

func (o *otlpOutput) String() string {
	b, err := json.Marshal(o.cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

// Init initializes the OTLP output
func (o *otlpOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, o.cfg)
	if err != nil {
		return err
	}

	if o.cfg.Name == "" {
		o.cfg.Name = name
	}
	o.logger.SetPrefix(fmt.Sprintf(loggingPrefix, o.cfg.Name))

	// Apply options
	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	// Set defaults
	o.setName(options.Name)
	err = o.setDefaults()
	if err != nil {
		return err
	}

	// Apply logger
	if options.Logger != nil {
		o.logger.SetOutput(options.Logger.Writer())
		o.logger.SetFlags(options.Logger.Flags())
	}

	// Initialize registry
	o.reg = options.Registry
	err = o.registerMetrics()
	if err != nil {
		return err
	}

	// Initialize event processors
	o.evps, err = formatters.MakeEventProcessors(options.Logger, o.cfg.EventProcessors,
		options.EventProcessors, options.TargetsConfig, options.Actions)
	if err != nil {
		return err
	}

	// Initialize transport
	if o.cfg.Protocol == "grpc" {
		err = o.initGRPC()
		if err != nil {
			return fmt.Errorf("failed to initialize gRPC transport: %w", err)
		}
	} else if o.cfg.Protocol == "http" {
		return fmt.Errorf("HTTP transport not yet implemented")
	} else {
		return fmt.Errorf("unsupported protocol '%s': must be 'grpc' or 'http'", o.cfg.Protocol)
	}

	// Initialize worker channels
	bufferSize := o.cfg.BufferSize
	if bufferSize == 0 {
		bufferSize = o.cfg.BatchSize * 2
	}
	o.eventCh = make(chan *formatters.EventMsg, bufferSize)

	// Start workers
	o.ctx, o.cancelFn = context.WithCancel(ctx)
	o.wg.Add(o.cfg.NumWorkers)
	for i := 0; i < o.cfg.NumWorkers; i++ {
		go o.worker(o.ctx, i)
	}

	o.logger.Printf("initialized OTLP output: endpoint=%s, protocol=%s, batch-size=%d, workers=%d",
		o.cfg.Endpoint, o.cfg.Protocol, o.cfg.BatchSize, o.cfg.NumWorkers)

	return nil
}

func (o *otlpOutput) setDefaults() error {
	if o.cfg.Timeout == 0 {
		o.cfg.Timeout = defaultTimeout
	}
	if o.cfg.BatchSize == 0 {
		o.cfg.BatchSize = defaultBatchSize
	}
	if o.cfg.NumWorkers == 0 {
		o.cfg.NumWorkers = defaultNumWorkers
	}
	if o.cfg.MaxRetries == 0 {
		o.cfg.MaxRetries = defaultMaxRetries
	}
	if o.cfg.Protocol == "" {
		o.cfg.Protocol = defaultProtocol
	}
	if o.cfg.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if o.cfg.Name == "" {
		o.cfg.Name = "gnmic-otlp-" + uuid.New().String()
	}
	if o.cfg.Interval == 0 {
		o.cfg.Interval = 5 * time.Second
	}

	return nil
}

func (o *otlpOutput) setName(name string) {
	if name != "" {
		o.cfg.Name = name
	}
}

// initGRPC initializes the gRPC connection to the OTLP collector
func (o *otlpOutput) initGRPC() error {
	var opts []grpc.DialOption

	if o.cfg.TLS != nil {
		tlsConfig, err := o.createTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to create TLS config: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(o.cfg.Endpoint, opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP client: %w", err)
	}

	o.grpcConn = conn
	o.grpcClient = metricsv1.NewMetricsServiceClient(conn)

	o.logger.Printf("initialized OTLP gRPC client for endpoint: %s", o.cfg.Endpoint)
	return nil
}

func (o *otlpOutput) createTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: o.cfg.TLS.SkipVerify,
	}

	if o.cfg.TLS.CaFile != "" || o.cfg.TLS.CertFile != "" {
		return utils.NewTLSConfig(
			o.cfg.TLS.CaFile,
			o.cfg.TLS.CertFile,
			o.cfg.TLS.KeyFile,
			"", // client auth
			o.cfg.TLS.SkipVerify,
			false,
		)
	}

	return tlsConfig, nil
}

// Write handles incoming gNMI messages
func (o *otlpOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	// Type assert to gNMI SubscribeResponse
	subsResp, ok := rsp.(*gnmi.SubscribeResponse)
	if !ok {
		if o.cfg.Debug {
			o.logger.Printf("received non-SubscribeResponse message, ignoring")
		}
		return
	}

	// Convert gNMI response to EventMsg format
	subscriptionName := meta["subscription-name"]
	if subscriptionName == "" {
		subscriptionName = "default"
	}

	events, err := formatters.ResponseToEventMsgs(subscriptionName, subsResp, meta, o.evps...)
	if err != nil {
		if o.cfg.Debug {
			o.logger.Printf("failed to convert response to events: %v", err)
		}
		return
	}

	// Send events to worker channel
	for _, event := range events {
		select {
		case o.eventCh <- event:
		case <-ctx.Done():
			return
		default:
			if o.cfg.Debug {
				o.logger.Printf("event channel full, dropping event")
			}
			if o.cfg.EnableMetrics {
				otlpNumberOfFailedEvents.WithLabelValues(o.cfg.Name, "channel_full").Inc()
			}
		}
	}
}

// WriteEvent handles individual EventMsg
func (o *otlpOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	if ev == nil {
		return
	}

	select {
	case o.eventCh <- ev:
	case <-ctx.Done():
		return
	default:
		if o.cfg.Debug {
			o.logger.Printf("event channel full, dropping event")
		}
		if o.cfg.EnableMetrics {
			otlpNumberOfFailedEvents.WithLabelValues(o.cfg.Name, "channel_full").Inc()
		}
	}
}

// Close closes the OTLP output
func (o *otlpOutput) Close() error {
	if o.cancelFn != nil {
		o.cancelFn()
	}

	// Close event channel
	close(o.eventCh)

	// Wait for workers to finish
	o.wg.Wait()

	// Close gRPC connection
	if o.grpcConn != nil {
		return o.grpcConn.Close()
	}

	return nil
}

// worker processes events in batches
func (o *otlpOutput) worker(ctx context.Context, id int) {
	defer o.wg.Done()

	if o.cfg.Debug {
		o.logger.Printf("worker %d started", id)
	}

	batch := make([]*formatters.EventMsg, 0, o.cfg.BatchSize)
	ticker := time.NewTicker(o.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Flush remaining batch with fresh context to avoid sending with cancelled context
			if len(batch) > 0 {
				flushCtx, cancel := context.WithTimeout(context.Background(), o.cfg.Timeout)
				defer cancel()
				o.sendBatch(flushCtx, batch)
			}
			if o.cfg.Debug {
				o.logger.Printf("worker %d stopped", id)
			}
			return

		case event, ok := <-o.eventCh:
			if !ok {
				// Channel closed, flush and exit with fresh context
				if len(batch) > 0 {
					flushCtx, cancel := context.WithTimeout(context.Background(), o.cfg.Timeout)
					defer cancel()
					o.sendBatch(flushCtx, batch)
				}
				return
			}

			batch = append(batch, event)
			if len(batch) >= o.cfg.BatchSize {
				o.sendBatch(ctx, batch)
				batch = make([]*formatters.EventMsg, 0, o.cfg.BatchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				o.sendBatch(ctx, batch)
				batch = make([]*formatters.EventMsg, 0, o.cfg.BatchSize)
			}
		}
	}
}

// sendBatch sends a batch of events to the OTLP collector
func (o *otlpOutput) sendBatch(ctx context.Context, events []*formatters.EventMsg) {
	if len(events) == 0 {
		return
	}

	start := time.Now()

	// Convert to OTLP format
	req := o.convertToOTLP(events)

	// Send with retries
	var err error
	for attempt := 0; attempt <= o.cfg.MaxRetries; attempt++ {
		err = o.sendGRPC(ctx, req)

		if err == nil {
			if o.cfg.Debug {
				o.logger.Printf("successfully sent %d events (attempt %d)", len(events), attempt+1)
			}
			if o.cfg.EnableMetrics {
				otlpNumberOfSentEvents.WithLabelValues(o.cfg.Name).Add(float64(len(events)))
				otlpSendDuration.WithLabelValues(o.cfg.Name).Observe(time.Since(start).Seconds())
			}
			return
		}

		if attempt < o.cfg.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}

	o.logger.Printf("failed to send batch after %d retries: %v", o.cfg.MaxRetries, err)
	if o.cfg.EnableMetrics {
		otlpNumberOfFailedEvents.WithLabelValues(o.cfg.Name, "send_failed").Add(float64(len(events)))
	}
}

// registerMetrics registers Prometheus metrics for the OTLP output
func (o *otlpOutput) registerMetrics() error {
	if !o.cfg.EnableMetrics {
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
