// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package tcp_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/outputs"
	gutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
)

const (
	defaultRetryTimer = 2 * time.Second
	defaultNumWorkers = 1
	defaultMaxRetries = 3
	outputType        = "tcp"
)

func init() {
	outputs.Register("tcp", func() outputs.Output {
		return &tcpOutput{}
	})
}

type tcpOutput struct {
	outputs.BaseOutput

	cfg      *atomic.Pointer[config]
	dynCfg   *atomic.Pointer[dynConfig]
	rootCtx  context.Context
	cancelFn context.CancelFunc
	wg       *sync.WaitGroup
	buffer   *atomic.Pointer[chan []byte]
	logger   *slog.Logger
	name     string
	reg      *prometheus.Registry

	store store.Store[any]
}

type dynConfig struct {
	targetTpl *template.Template
	evps      []formatters.EventProcessor
	mo        *formatters.MarshalOptions
	delimiter []byte
	limiter   *time.Ticker
}

type config struct {
	Address            string        `mapstructure:"address,omitempty"` // ip:port
	Rate               time.Duration `mapstructure:"rate,omitempty"`
	BufferSize         uint          `mapstructure:"buffer-size,omitempty"`
	Format             string        `mapstructure:"format,omitempty"`
	AddTarget          string        `mapstructure:"add-target,omitempty"`
	TargetTemplate     string        `mapstructure:"target-template,omitempty"`
	OverrideTimestamps bool          `mapstructure:"override-timestamps,omitempty"`
	SplitEvents        bool          `mapstructure:"split-events,omitempty"`
	Delimiter          string        `mapstructure:"delimiter,omitempty"`
	KeepAlive          time.Duration `mapstructure:"keep-alive,omitempty"`
	RetryInterval      time.Duration `mapstructure:"retry-interval,omitempty"`
	MaxRetries         int           `mapstructure:"max-retries,omitempty"`
	NumWorkers         int           `mapstructure:"num-workers,omitempty"`
	EnableMetrics      bool          `mapstructure:"enable-metrics,omitempty"`
	EventProcessors    []string      `mapstructure:"event-processors,omitempty"`
}

func (t *tcpOutput) buildEventProcessors(logger *slog.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(t.store)
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

func (t *tcpOutput) init() {
	t.cfg = new(atomic.Pointer[config])
	t.dynCfg = new(atomic.Pointer[dynConfig])
	t.buffer = new(atomic.Pointer[chan []byte])
	t.wg = new(sync.WaitGroup)
	t.logger = logging.DiscardLogger()
}

func (t *tcpOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	t.init()
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	setDefaultsFor(newCfg)
	t.cfg.Store(newCfg)

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	t.store = options.Store
	t.name = name
	t.reg = options.Registry

	t.logger = outputs.BindLogger(options.Logger, outputType, name)

	dc := new(dynConfig)
	// initialize event processors
	dc.evps, err = t.buildEventProcessors(t.logger, newCfg.EventProcessors)
	if err != nil {
		return err
	}
	dc.mo = &formatters.MarshalOptions{
		Format:     newCfg.Format,
		OverrideTS: newCfg.OverrideTimestamps,
	}
	if newCfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	_, _, err = net.SplitHostPort(newCfg.Address)
	if err != nil {
		return fmt.Errorf("wrong address format: %v", err)
	}
	if err := t.registerMetrics(newCfg); err != nil {
		return err
	}
	ch := make(chan []byte, newCfg.BufferSize)
	t.buffer.Store(&ch)
	if newCfg.Rate > 0 {
		dc.limiter = time.NewTicker(newCfg.Rate)
	}
	if len(newCfg.Delimiter) > 0 {
		dc.delimiter = []byte(newCfg.Delimiter)
	}

	t.dynCfg.Store(dc)
	t.cfg.Store(newCfg)
	t.rootCtx = ctx
	ctx, t.cancelFn = context.WithCancel(t.rootCtx)
	t.wg.Add(newCfg.NumWorkers)
	for i := 0; i < newCfg.NumWorkers; i++ {
		go t.start(ctx, t.wg, i)
	}
	return nil
}

func setDefaultsFor(cfg *config) {
	if cfg.RetryInterval == 0 {
		cfg.RetryInterval = defaultRetryTimer
	}
	if cfg.NumWorkers < 1 {
		cfg.NumWorkers = defaultNumWorkers
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
}

func validate(cfg *config) error {
	if cfg.MaxRetries < 0 {
		return errors.New("max-retries must be non-negative")
	}
	if cfg.Address == "" {
		return errors.New("address is required")
	}
	_, _, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return fmt.Errorf("wrong address format: %v", err)
	}
	if cfg.TargetTemplate == "" {
		return errors.New("target-template is required")
	}
	return nil
}

func (t *tcpOutput) Validate(cfg map[string]any) error {
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	setDefaultsFor(newCfg)
	return validate(newCfg)
}

func (t *tcpOutput) Update(_ context.Context, cfg map[string]any) error {
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	setDefaultsFor(newCfg)
	currCfg := t.cfg.Load()

	if newCfg.EnableMetrics && (currCfg == nil || !currCfg.EnableMetrics) {
		if err := t.registerMetrics(newCfg); err != nil {
			return err
		}
	}

	swapChannel := channelNeedsSwap(currCfg, newCfg)
	restartWorkers := needsWorkerRestart(currCfg, newCfg)
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0

	dc := new(dynConfig)
	prevDC := t.dynCfg.Load()
	if rebuildProcessors {
		dc.evps, err = t.buildEventProcessors(t.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}
	dc.delimiter = []byte(newCfg.Delimiter)
	if newCfg.Rate > 0 {
		// if rate changed
		if currCfg.Rate != newCfg.Rate {
			if prevDC != nil && prevDC.limiter != nil {
				prevDC.limiter.Stop()
			}
			dc.limiter = time.NewTicker(newCfg.Rate)
		} else {
			dc.limiter = prevDC.limiter
		}
	} else if prevDC != nil && prevDC.limiter != nil { // stop old limiter if any
		prevDC.limiter.Stop()
	}
	dc.mo = &formatters.MarshalOptions{
		Format:     newCfg.Format,
		OverrideTS: newCfg.OverrideTimestamps,
	}

	if newCfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	} else {
		dc.targetTpl = outputs.DefaultTargetTemplate
	}
	t.dynCfg.Store(dc)
	t.cfg.Store(newCfg)
	if swapChannel || restartWorkers {
		// only reassign wg/cancelFn when actually restarting workers,
		// and pass the new wg into the new goroutines so each Done()
		// targets the wg it was started with.
		oldChan := *t.buffer.Load()
		oldWg := t.wg
		oldCancel := t.cancelFn
		newWg := new(sync.WaitGroup)
		t.wg = newWg

		var newChan chan []byte
		if swapChannel {
			newChan = make(chan []byte, newCfg.BufferSize)
		} else {
			newChan = oldChan
		}
		// swap channel
		t.buffer.Store(&newChan)

		var ctx context.Context
		ctx, t.cancelFn = context.WithCancel(t.rootCtx)
		newWg.Add(newCfg.NumWorkers)
		for i := 0; i < newCfg.NumWorkers; i++ {
			go t.start(ctx, newWg, i)
		}
		if oldCancel != nil {
			oldCancel()
		}
		if oldWg != nil {
			oldWg.Wait()
		}
		if swapChannel {
		DRAIN_LOOP:
			for {
				select {
				case b, ok := <-oldChan:
					if !ok {
						break
					}
					select {
					case newChan <- b:
					default:
						// new channel full, drop message
					}
				default:
					break DRAIN_LOOP
				}
			}
		}
		t.logger.Info("restarted TCP output workers")
	} else {
		t.logger.Debug("no changes to TCP output")
	}
	t.logger.Info("updated TCP output", slog.Any("config", t.String()))
	return nil
}

func (t *tcpOutput) UpdateProcessor(name string, pcfg map[string]any) error {
	cfg := t.cfg.Load()
	dc := t.dynCfg.Load()

	newEvps, changed, err := outputs.UpdateProcessorInSlice(
		t.logger,
		t.store,
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
		t.dynCfg.Store(&newDC)
		t.logger.Info("updated event processor", "name", name)
	}
	return nil
}

func (t *tcpOutput) Write(ctx context.Context, m proto.Message, meta outputs.Meta) {
	if m == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
		cfg := t.cfg.Load()
		dc := t.dynCfg.Load()
		rsp, err := outputs.AddSubscriptionTarget(m, meta, cfg.AddTarget, dc.targetTpl)
		if err != nil {
			t.logger.Warn("failed to add target to response", "err", err)
		}
		bb, err := outputs.Marshal(rsp, meta, dc.mo, cfg.SplitEvents, dc.evps...)
		if err != nil {
			t.logger.Warn("failed marshaling proto msg", "err", err)
			return
		}
		buffer := t.buffer.Load()
		for _, b := range bb {
			(*buffer) <- b
		}
	}
}

func (t *tcpOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

func (t *tcpOutput) Close() error {
	t.cancelFn()
	t.wg.Wait()
	dc := t.dynCfg.Load()
	if dc != nil && dc.limiter != nil {
		dc.limiter.Stop()
	}
	return nil
}

func (t *tcpOutput) String() string {
	cfg := t.cfg.Load()
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

type tcpDialFunc func(context.Context, string) (net.Conn, error)

func tcpDialContext(ctx context.Context, address string) (net.Conn, error) {
	// Keep keepalive disabled unless the output configuration enables it.
	dialer := net.Dialer{KeepAlive: -1}
	return dialer.DialContext(ctx, "tcp", address)
}

func waitTCPRetry(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func writeTCPPayload(w io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := w.Write(payload)
		if n > 0 {
			payload = payload[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}

func (t *tcpOutput) recordTCPError(cfg *config, reason string) {
	if cfg.EnableMetrics {
		tcpOutputErrors.WithLabelValues(t.name, reason).Inc()
	}
}

func (t *tcpOutput) pendingRetryExhausted(
	cfg *config,
	worker string,
	reason string,
	retries *int,
) bool {
	(*retries)++
	if *retries <= cfg.MaxRetries {
		return false
	}

	t.logger.Error(
		"dropping TCP message after retry limit",
		"worker",
		worker,
		"attempts",
		*retries,
		"max-retries",
		cfg.MaxRetries,
		"reason",
		reason,
	)
	if cfg.EnableMetrics {
		tcpOutputDroppedMessages.WithLabelValues(t.name, "max_retries").Inc()
	}
	return true
}

func (t *tcpOutput) start(ctx context.Context, wg *sync.WaitGroup, idx int) {
	t.startWithDialer(ctx, wg, idx, tcpDialContext)
}

func (t *tcpOutput) startWithDialer(
	ctx context.Context,
	wg *sync.WaitGroup,
	idx int,
	dial tcpDialFunc,
) {
	defer wg.Done()

	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	buffer := *t.buffer.Load()

	var (
		conn           net.Conn
		pending        []byte
		pendingRetries int
	)
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	for {
		if ctx.Err() != nil {
			return
		}

		cfg := t.cfg.Load()
		if conn == nil {
			var err error
			conn, err = dial(ctx, cfg.Address)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				t.logger.Error(
					"failed to dial TCP",
					"worker",
					workerLogPrefix,
					"err",
					err,
				)
				t.recordTCPError(cfg, "dial")
				if pending != nil && t.pendingRetryExhausted(
					cfg,
					workerLogPrefix,
					"dial",
					&pendingRetries,
				) {
					pending = nil
					pendingRetries = 0
				}
				if !waitTCPRetry(ctx, cfg.RetryInterval) {
					return
				}
				continue
			}

			if tcpConn, ok := conn.(*net.TCPConn); ok && cfg.KeepAlive > 0 {
				_ = tcpConn.SetKeepAlive(true)
				_ = tcpConn.SetKeepAlivePeriod(cfg.KeepAlive)
			}
		}

		if pending == nil {
			select {
			case <-ctx.Done():
				return
			case b := <-buffer:
				dc := t.dynCfg.Load()
				if dc.limiter != nil {
					select {
					case <-ctx.Done():
						return
					case <-dc.limiter.C:
					}
				}

				pending = make(
					[]byte,
					0,
					len(b)+len(dc.delimiter),
				)
				pending = append(pending, b...)
				pending = append(pending, dc.delimiter...)
				pendingRetries = 0
			}
		}

		if err := writeTCPPayload(conn, pending); err != nil {
			t.logger.Error(
				"failed sending tcp bytes",
				"worker",
				workerLogPrefix,
				"err",
				err,
			)
			t.recordTCPError(cfg, "write")

			_ = conn.Close()
			conn = nil

			if t.pendingRetryExhausted(
				cfg,
				workerLogPrefix,
				"write",
				&pendingRetries,
			) {
				pending = nil
				pendingRetries = 0
			}
			if !waitTCPRetry(ctx, cfg.RetryInterval) {
				return
			}
			continue
		}

		pending = nil
		pendingRetries = 0
	}
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
	return old.NumWorkers != nw.NumWorkers
}
