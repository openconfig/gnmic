// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package clickhouse_output

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/outputs"
	pkgutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
)

const (
	outputType = "clickhouse"

	defaultAddress          = "localhost:9000"
	defaultDatabase         = "default"
	defaultTable            = "gnmic_telemetry"
	defaultUsername         = "default"
	defaultBatchSize        = 1000
	defaultFlushTimer       = 5 * time.Second
	defaultTimeout          = 5 * time.Second
	defaultRecoveryWaitTime = 10 * time.Second
	defaultNumWorkers       = 1
	defaultBufferSize       = 10000
	defaultMaxInFlight      = 1
	defaultTableEngine      = "MergeTree"
	defaultPartitionBy      = "toYYYYMMDD(timestamp)"
)

// insertBatchHook is set by tests only to mock batch inserts.
var insertBatchHook func(ctx context.Context, conn driver.Conn, cfg *config, rows []telemetryRow) error

// openConnHook is set by tests only to mock TCP dial / client construction.
var openConnHook func(*config) (driver.Conn, error)

func init() {
	outputs.Register(outputType, func() outputs.Output {
		return &clickhouseOutput{}
	})
}

type clickhouseOutput struct {
	outputs.BaseOutput

	cfg     *atomic.Pointer[config]
	dynCfg  *atomic.Pointer[dynConfig]
	rowChan *atomic.Pointer[chan telemetryRow]

	conn   *atomic.Pointer[driver.Conn]
	logger *slog.Logger
	reg    *prometheus.Registry
	store  store.Store[any]

	rootCtx   context.Context
	cancelFn  context.CancelFunc
	wg        *sync.WaitGroup
	closeOnce sync.Once
	closeSig  chan struct{}

	insertMu  sync.Mutex
	rowSendMu sync.Mutex // serializes channel close/drain in Update with enqueue sends
}

type dynConfig struct {
	targetTpl *template.Template
	evps      []formatters.EventProcessor
}

type config struct {
	Name               string           `mapstructure:"name,omitempty"`
	Address            string           `mapstructure:"address,omitempty"`
	Database           string           `mapstructure:"database,omitempty"`
	Table              string           `mapstructure:"table,omitempty"`
	Username           string           `mapstructure:"username,omitempty"`
	Password           string           `mapstructure:"password,omitempty"`
	TLS                *types.TLSConfig `mapstructure:"tls,omitempty"`
	AddTarget          string           `mapstructure:"add-target,omitempty"`
	TargetTemplate     string           `mapstructure:"target-template,omitempty"`
	OverrideTimestamps bool             `mapstructure:"override-timestamps,omitempty"`
	EventProcessors    []string         `mapstructure:"event-processors,omitempty"`
	BatchSize          int              `mapstructure:"batch-size,omitempty"`
	FlushTimer         time.Duration    `mapstructure:"flush-timer,omitempty"`
	MaxInFlight        int              `mapstructure:"max-in-flight,omitempty"`
	NumWorkers         int              `mapstructure:"num-workers,omitempty"`
	BufferSize         int              `mapstructure:"buffer-size,omitempty"`
	Timeout            time.Duration    `mapstructure:"timeout,omitempty"`
	RecoveryWaitTime   time.Duration    `mapstructure:"recovery-wait-time,omitempty"`
	CreateTable        bool             `mapstructure:"create-table,omitempty"`
	TableEngine        string           `mapstructure:"table-engine,omitempty"`
	PartitionBy        string           `mapstructure:"partition-by,omitempty"`
	OrderBy            []string         `mapstructure:"order-by,omitempty"`
	TTL                string           `mapstructure:"ttl,omitempty"`
	Debug              bool             `mapstructure:"debug,omitempty"`
	EnableMetrics      bool             `mapstructure:"enable-metrics,omitempty"`
}

func (c *config) LogValue() slog.Value {
	return logging.RedactedValue(c)
}

func (o *clickhouseOutput) init() {
	o.cfg = new(atomic.Pointer[config])
	o.dynCfg = new(atomic.Pointer[dynConfig])
	o.rowChan = new(atomic.Pointer[chan telemetryRow])
	o.conn = new(atomic.Pointer[driver.Conn])
	o.wg = new(sync.WaitGroup)
	o.logger = logging.DiscardLogger()
	o.closeSig = make(chan struct{})
	var nilConn driver.Conn
	o.conn.Store(&nilConn)
}

func (o *clickhouseOutput) String() string {
	cfg := o.cfg.Load()
	if cfg == nil {
		return ""
	}
	return logging.RedactedJSON(cfg)
}

func (o *clickhouseOutput) buildEventProcessors(logger *slog.Logger, names []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := pkgutils.GetConfigMaps(o.store)
	if err != nil {
		return nil, err
	}
	return formatters.MakeEventProcessors(logger, names, ps, tcs, acts)
}

func setDefaults(c *config) error {
	if c.Address == "" {
		c.Address = defaultAddress
	}
	if c.Database == "" {
		c.Database = defaultDatabase
	}
	if c.Table == "" {
		c.Table = defaultTable
	}
	if c.Username == "" {
		c.Username = defaultUsername
	}
	if c.BatchSize <= 0 {
		c.BatchSize = defaultBatchSize
	}
	if c.FlushTimer <= 0 {
		c.FlushTimer = defaultFlushTimer
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.RecoveryWaitTime <= 0 {
		c.RecoveryWaitTime = defaultRecoveryWaitTime
	}
	if c.NumWorkers <= 0 {
		c.NumWorkers = defaultNumWorkers
	}
	if c.BufferSize <= 0 {
		c.BufferSize = defaultBufferSize
	}
	if c.MaxInFlight <= 0 {
		c.MaxInFlight = defaultMaxInFlight
	}
	if c.TableEngine == "" {
		c.TableEngine = defaultTableEngine
	}
	if strings.TrimSpace(c.PartitionBy) == "" {
		c.PartitionBy = defaultPartitionBy
	}
	if len(c.OrderBy) == 0 {
		c.OrderBy = []string{"target", "path", "timestamp"}
	}
	return nil
}

func applyTTLDefault(raw map[string]any, c *config) {
	if raw != nil {
		if _, has := raw["ttl"]; has && strings.TrimSpace(c.TTL) == "" {
			return
		}
	}
	if strings.TrimSpace(c.TTL) == "" {
		c.TTL = "30 DAY"
	}
}

func applyCreateTableDefault(raw map[string]any, c *config) {
	if raw != nil {
		if _, has := raw["create-table"]; has {
			return
		}
	}
	c.CreateTable = true
}

func (o *clickhouseOutput) Init(ctx context.Context, name string, cfg map[string]any, opts ...outputs.Option) error {
	o.init()
	ncfg := new(config)
	if err := outputs.DecodeConfig(cfg, ncfg); err != nil {
		return err
	}
	if ncfg.Name == "" {
		ncfg.Name = name
	}
	applyTTLDefault(cfg, ncfg)
	applyCreateTableDefault(cfg, ncfg)
	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}
	o.store = options.Store
	o.logger = outputs.BindLogger(options.Logger, outputType, ncfg.Name)
	o.reg = options.Registry

	if err := setDefaults(ncfg); err != nil {
		return err
	}
	o.cfg.Store(ncfg)

	if err := o.registerMetrics(); err != nil {
		return err
	}
	o.initMetricLabels()

	dc := new(dynConfig)
	var err error
	dc.evps, err = o.buildEventProcessors(o.logger, ncfg.EventProcessors)
	if err != nil {
		return err
	}
	if ncfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if ncfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", ncfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}
	o.dynCfg.Store(dc)

	o.rootCtx = ctx

	ch := make(chan telemetryRow, ncfg.BufferSize)
	o.rowChan.Store(&ch)

	runCtx, cancel := context.WithCancel(o.rootCtx)
	o.cancelFn = cancel

	o.wg.Add(1 + ncfg.NumWorkers)
	go o.runBootstrapDial(runCtx)
	for i := 0; i < ncfg.NumWorkers; i++ {
		go o.worker(runCtx, i)
	}

	o.logger.Info("initialized clickhouse output", slog.Any("config", o.String()))
	return nil
}

func openConn(c *config) (driver.Conn, error) {
	opt := &clickhouse.Options{
		Addr:         []string{c.Address},
		DialTimeout:  c.Timeout,
		MaxOpenConns: c.MaxInFlight + 2,
		Auth: clickhouse.Auth{
			Database: c.Database,
			Username: c.Username,
			Password: c.Password,
		},
	}
	if c.TLS != nil {
		tlsCfg, err := utils.NewTLSConfig(
			c.TLS.CaFile,
			c.TLS.CertFile,
			c.TLS.KeyFile,
			"",
			c.TLS.SkipVerify,
			false,
		)
		if err != nil {
			return nil, err
		}
		if tlsCfg == nil {
			return nil, fmt.Errorf("clickhouse: TLS enabled but no TLS settings applied (set tls.ca-file or tls.cert-file+tls.key-file, or tls.skip-verify: true)")
		}
		opt.TLS = tlsCfg
	}
	return clickhouse.Open(opt)
}

func tryOpenConn(cfg *config) (driver.Conn, error) {
	if openConnHook != nil {
		return openConnHook(cfg)
	}
	return openConn(cfg)
}

// runBootstrapDial connects in the background until dialUntilReady succeeds or ctx ends.
func (o *clickhouseOutput) runBootstrapDial(ctx context.Context) {
	defer o.wg.Done()
	conn, err := o.dialUntilReady(ctx)
	if err != nil {
		if ctx.Err() != nil {
			o.logger.Debug("clickhouse bootstrap dial stopped", "reason", ctx.Err())
			return
		}
		o.logger.Error("clickhouse bootstrap dial failed", "err", err)
		return
	}
	old := o.conn.Swap(&conn)
	if old != nil && *old != nil {
		_ = (*old).Close()
	}
	o.logger.Info("clickhouse connection ready")
}

// dialUntilReady opens a connection, pings ClickHouse, and optionally creates the
// telemetry table, retrying with recovery-wait-time until success or ctx is done.
// It reloads o.cfg on each attempt so address/credentials updates apply while dialing.
func (o *clickhouseOutput) dialUntilReady(ctx context.Context) (driver.Conn, error) {
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return nil, fmt.Errorf("clickhouse bootstrap cancelled: %w (last error: %v)", err, lastErr)
			}
			return nil, fmt.Errorf("clickhouse bootstrap cancelled: %w", err)
		}

		cfg := o.cfg.Load()
		if cfg == nil {
			return nil, fmt.Errorf("clickhouse: no config loaded")
		}

		conn, err := tryOpenConn(cfg)
		if err != nil {
			lastErr = err
			o.logger.Warn("clickhouse connect failed, will retry", "err", err, "wait", cfg.RecoveryWaitTime)
			if err := wait(ctx, cfg.RecoveryWaitTime); err != nil {
				return nil, fmt.Errorf("clickhouse connect: %w (last dial error: %v)", err, lastErr)
			}
			continue
		}

		pingCtx, cancelPing := context.WithTimeout(ctx, cfg.Timeout)
		err = conn.Ping(pingCtx)
		cancelPing()
		if err != nil {
			lastErr = err
			_ = conn.Close()
			o.logger.Warn("clickhouse ping failed, will retry", "err", err, "wait", cfg.RecoveryWaitTime)
			if err := wait(ctx, cfg.RecoveryWaitTime); err != nil {
				return nil, fmt.Errorf("clickhouse ping: %w (last ping error: %v)", err, lastErr)
			}
			continue
		}

		if cfg.CreateTable {
			ddl, err := buildCreateTableSQL(cfg)
			if err != nil {
				_ = conn.Close()
				return nil, err
			}
			execCtx, cancelExec := context.WithTimeout(ctx, cfg.Timeout)
			err = conn.Exec(execCtx, ddl)
			cancelExec()
			if err != nil {
				lastErr = err
				_ = conn.Close()
				o.logger.Warn("clickhouse create table failed, will retry", "err", err, "wait", cfg.RecoveryWaitTime)
				if err := wait(ctx, cfg.RecoveryWaitTime); err != nil {
					return nil, fmt.Errorf("clickhouse create table: %w (last exec error: %v)", err, lastErr)
				}
				continue
			}
		}

		return conn, nil
	}
}

func wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		d = time.Second
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// waitLiveConn blocks until a non-nil connection is stored, ctx ends, or cfg is removed.
func (o *clickhouseOutput) waitLiveConn(ctx context.Context, cfg *config, batchLen int) bool {
	poll := connPollInterval(cfg)
	for {
		p := o.conn.Load()
		if p != nil && *p != nil {
			return true
		}
		if err := ctx.Err(); err != nil {
			if cfg.EnableMetrics {
				chFailedRows.WithLabelValues(cfg.Name, cfg.Table, "no_connection").Add(float64(batchLen))
			}
			return false
		}
		if err := wait(ctx, poll); err != nil {
			if cfg.EnableMetrics {
				chFailedRows.WithLabelValues(cfg.Name, cfg.Table, "no_connection").Add(float64(batchLen))
			}
			return false
		}
		cfg = o.cfg.Load()
		if cfg == nil {
			return false
		}
	}
}

func connPollInterval(cfg *config) time.Duration {
	d := cfg.RecoveryWaitTime / 10
	if d < 100*time.Millisecond {
		d = 100 * time.Millisecond
	}
	if d > time.Second {
		d = time.Second
	}
	return d
}

func (o *clickhouseOutput) Validate(cfg map[string]any) error {
	ncfg := new(config)
	if err := outputs.DecodeConfig(cfg, ncfg); err != nil {
		return err
	}
	applyTTLDefault(cfg, ncfg)
	applyCreateTableDefault(cfg, ncfg)
	if err := setDefaults(ncfg); err != nil {
		return err
	}
	if _, err := buildCreateTableSQL(ncfg); err != nil {
		return err
	}
	if ncfg.TargetTemplate != "" {
		if _, err := gtemplate.CreateTemplate("target-template", ncfg.TargetTemplate); err != nil {
			return err
		}
	}
	return nil
}

func (o *clickhouseOutput) Update(ctx context.Context, cfg map[string]any) error {
	newCfg := new(config)
	if err := outputs.DecodeConfig(cfg, newCfg); err != nil {
		return err
	}
	applyTTLDefault(cfg, newCfg)
	applyCreateTableDefault(cfg, newCfg)
	curr := o.cfg.Load()
	if newCfg.Name == "" && curr != nil {
		newCfg.Name = curr.Name
	}
	if err := setDefaults(newCfg); err != nil {
		return err
	}

	rebuildProcessors := curr == nil || slices.Compare(curr.EventProcessors, newCfg.EventProcessors) != 0

	var targetTpl *template.Template
	var err error
	if newCfg.TargetTemplate == "" {
		targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		targetTpl = targetTpl.Funcs(outputs.TemplateFuncs)
	} else {
		targetTpl = outputs.DefaultTargetTemplate
	}

	dc := &dynConfig{targetTpl: targetTpl}
	prevDC := o.dynCfg.Load()
	if rebuildProcessors {
		dc.evps, err = o.buildEventProcessors(o.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}
	o.dynCfg.Store(dc)

	swapChan := curr == nil || curr.BufferSize != newCfg.BufferSize
	restart := curr == nil ||
		curr.Address != newCfg.Address ||
		curr.Database != newCfg.Database ||
		curr.Table != newCfg.Table ||
		curr.Username != newCfg.Username ||
		curr.Password != newCfg.Password ||
		!tlsEqual(curr.TLS, newCfg.TLS) ||
		curr.NumWorkers != newCfg.NumWorkers ||
		swapChan ||
		curr.CreateTable != newCfg.CreateTable ||
		curr.TableEngine != newCfg.TableEngine ||
		curr.PartitionBy != newCfg.PartitionBy ||
		!slices.Equal(curr.OrderBy, newCfg.OrderBy) ||
		curr.TTL != newCfg.TTL

	o.cfg.Store(newCfg)
	o.initMetricLabels()

	if !restart {
		o.logger.Info("updated clickhouse output", slog.Any("config", o.String()))
		return nil
	}

	if o.rootCtx == nil {
		return fmt.Errorf("clickhouse output: restart requested before Init completed")
	}
	_ = ctx // Update request context; dial and workers use o.rootCtx

	var newCh chan telemetryRow
	if swapChan {
		newCh = make(chan telemetryRow, newCfg.BufferSize)
	} else {
		newCh = *o.rowChan.Load()
	}

	oldCancel := o.cancelFn
	oldWG := o.wg
	oldCh := *o.rowChan.Load()

	if oldCancel != nil {
		oldCancel()
	}
	if oldWG != nil {
		oldWG.Wait()
	}

	if swapChan {
		o.rowSendMu.Lock()
		close(oldCh)
		o.drainRowChan(oldCh, newCh, newCfg)
		o.rowChan.Store(&newCh)
		o.rowSendMu.Unlock()
	}

	if p := o.conn.Load(); p != nil && *p != nil {
		_ = (*p).Close()
	}
	var nilConn driver.Conn
	o.conn.Store(&nilConn)

	runCtx, cancel := context.WithCancel(o.rootCtx)
	o.cancelFn = cancel
	o.wg = new(sync.WaitGroup)
	if !swapChan {
		o.rowChan.Store(&newCh)
	}

	o.wg.Add(1 + newCfg.NumWorkers)
	go o.runBootstrapDial(runCtx)
	for i := 0; i < newCfg.NumWorkers; i++ {
		go o.worker(runCtx, i)
	}

	o.logger.Info("updated clickhouse output", slog.Any("config", o.String()))
	return nil
}

// drainRowChan moves buffered rows from oldCh to newCh after workers on oldCh have stopped.
func (o *clickhouseOutput) drainRowChan(oldCh, newCh chan telemetryRow, cfg *config) {
	for {
		select {
		case <-o.rootCtx.Done():
			return
		case r, ok := <-oldCh:
			if !ok {
				return
			}
			select {
			case newCh <- r:
			default:
				if cfg.EnableMetrics {
					chDroppedRows.WithLabelValues(cfg.Name, cfg.Table, "drain_overflow").Inc()
				}
			}
		}
	}
}

func tlsEqual(a, b *types.TLSConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.CaFile == b.CaFile &&
		a.CertFile == b.CertFile &&
		a.KeyFile == b.KeyFile &&
		a.SkipVerify == b.SkipVerify
}

func (o *clickhouseOutput) UpdateProcessor(name string, pcfg map[string]any) error {
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
		nd := *dc
		nd.evps = newEvps
		o.dynCfg.Store(&nd)
	}
	return nil
}

func (o *clickhouseOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	cfg := o.cfg.Load()
	dc := o.dynCfg.Load()
	chPtr := o.rowChan.Load()
	if cfg == nil || dc == nil || chPtr == nil {
		return
	}

	var err error
	rsp, err = outputs.AddSubscriptionTarget(rsp, meta, cfg.AddTarget, dc.targetTpl)
	if err != nil {
		o.logger.Warn("failed to add target to response", "err", err)
	}

	if cfg.OverrideTimestamps {
		rsp = (&formatters.MarshalOptions{OverrideTS: true}).OverrideTimestamp(rsp)
	}

	var events []*formatters.EventMsg
	switch m := rsp.(type) {
	case *gnmi.SubscribeResponse:
		sub := "default"
		if sn, ok := meta["subscription-name"]; ok {
			sub = sn
		}
		events, err = formatters.ResponseToEventMsgs(sub, m, meta, dc.evps...)
		if err != nil {
			o.logger.Warn("failed to convert subscribe response to events", "err", err)
			return
		}
	case *gnmi.GetResponse:
		events, err = formatters.GetResponseToEventMsgs(m, meta, dc.evps...)
		if err != nil {
			o.logger.Warn("failed to convert get response to events", "err", err)
			return
		}
	default:
		return
	}

	for _, ev := range events {
		o.enqueueRows(ctx, cfg, ev, meta)
	}
}

func (o *clickhouseOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	cfg := o.cfg.Load()
	dc := o.dynCfg.Load()
	chPtr := o.rowChan.Load()
	if cfg == nil || dc == nil || chPtr == nil || ev == nil {
		return
	}

	evs := []*formatters.EventMsg{ev}
	for _, proc := range dc.evps {
		evs = proc.Apply(evs...)
	}
	for _, pev := range evs {
		o.enqueueRows(ctx, cfg, pev, nil)
	}
}

func (o *clickhouseOutput) enqueueRows(ctx context.Context, cfg *config, ev *formatters.EventMsg, meta outputs.Meta) {
	rows := eventToRows(ev, meta, cfg.OverrideTimestamps)
	for _, row := range rows {
		if cfg.EnableMetrics {
			chReceivedRows.WithLabelValues(cfg.Name, cfg.Table).Inc()
		}
		o.rowSendMu.Lock()
		chPtr := o.rowChan.Load()
		if chPtr == nil {
			o.rowSendMu.Unlock()
			return
		}
		ch := *chPtr
		wctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		select {
		case <-ctx.Done():
			cancel()
			o.rowSendMu.Unlock()
			return
		case <-o.closeSig:
			cancel()
			o.rowSendMu.Unlock()
			return
		case ch <- row:
			cancel()
			o.rowSendMu.Unlock()
		case <-wctx.Done():
			cancel()
			o.rowSendMu.Unlock()
			if cfg.Debug {
				o.logger.Warn("clickhouse output row enqueue timed out", "timeout", cfg.Timeout)
			}
			if cfg.EnableMetrics {
				chDroppedRows.WithLabelValues(cfg.Name, cfg.Table, "enqueue_timeout").Inc()
			}
		}
	}
}

func (o *clickhouseOutput) worker(ctx context.Context, id int) {
	defer o.wg.Done()
	prefix := fmt.Sprintf("worker-%d", id)
	o.logger.Info("clickhouse worker started", "worker", prefix)

	cfg := o.cfg.Load()
	batch := make([]telemetryRow, 0, cfg.BatchSize)
	ticker := time.NewTicker(cfg.FlushTimer)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		o.flushBatch(ctx, batch)
		batch = batch[:0]
	}

	chPtr := o.rowChan.Load()
	ch := *chPtr

	for {
		select {
		case <-ctx.Done():
			for {
				select {
				case r, ok := <-ch:
					if !ok {
						flush()
						o.logger.Info("clickhouse worker stopped", "worker", prefix)
						return
					}
					cfg = o.cfg.Load()
					if cfg == nil {
						continue
					}
					batch = append(batch, r)
					if len(batch) >= cfg.BatchSize {
						flush()
					}
				default:
					flush()
					o.logger.Info("clickhouse worker stopped", "worker", prefix)
					return
				}
			}
		case r, ok := <-ch:
			if !ok {
				flush()
				return
			}
			cfg = o.cfg.Load()
			if cfg.EnableMetrics {
				chBufferLen.WithLabelValues(cfg.Name, cfg.Table).Set(float64(len(ch)))
			}
			batch = append(batch, r)
			if len(batch) >= cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (o *clickhouseOutput) flushBatch(ctx context.Context, batch []telemetryRow) {
	if len(batch) == 0 {
		return
	}
	cfg := o.cfg.Load()
	if cfg == nil {
		return
	}
	if !o.waitLiveConn(ctx, cfg, len(batch)) {
		return
	}

	connPtr := o.conn.Load()
	if connPtr == nil || *connPtr == nil {
		return
	}

	for len(batch) > 0 {
		if ctx.Err() != nil {
			return
		}
		n := min(len(batch), cfg.BatchSize)
		chunk := batch[:n]
		batch = batch[n:]

		start := time.Now()
		for {
			if ctx.Err() != nil {
				if cfg.EnableMetrics {
					chFailedRows.WithLabelValues(cfg.Name, cfg.Table, "shutdown").Add(float64(len(chunk)))
				}
				return
			}
			insertCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			o.insertMu.Lock()
			err := insertBatchCH(insertCtx, *connPtr, cfg, chunk)
			o.insertMu.Unlock()
			cancel()
			if err == nil {
				break
			}
			o.logger.Error("clickhouse batch insert failed", "err", err)
			o.logger.Warn("reconnecting to clickhouse after insert failure", "wait", cfg.RecoveryWaitTime)
			time.Sleep(cfg.RecoveryWaitTime)
			newConn, cerr := tryOpenConn(cfg)
			if cerr != nil {
				o.logger.Error("clickhouse reconnect failed", "err", cerr)
				continue
			}
			if pingErr := newConn.Ping(ctx); pingErr != nil {
				o.logger.Error("clickhouse ping after reconnect failed", "err", pingErr)
				_ = newConn.Close()
				continue
			}
			old := o.conn.Swap(&newConn)
			if old != nil && *old != nil {
				_ = (*old).Close()
			}
			connPtr = o.conn.Load()
		}

		if cfg.EnableMetrics {
			chInsertBatches.WithLabelValues(cfg.Name, cfg.Table).Inc()
			chInsertBatchSize.WithLabelValues(cfg.Name, cfg.Table).Observe(float64(len(chunk)))
			chInsertDuration.WithLabelValues(cfg.Name, cfg.Table).Observe(time.Since(start).Seconds())
			chInsertedRows.WithLabelValues(cfg.Name, cfg.Table).Add(float64(len(chunk)))
		}
	}
}

func insertBatchCH(ctx context.Context, conn driver.Conn, cfg *config, rows []telemetryRow) error {
	if insertBatchHook != nil {
		return insertBatchHook(ctx, conn, cfg, rows)
	}
	if len(rows) == 0 {
		return nil
	}
	tbl := quoteIdent(cfg.Database) + "." + quoteIdent(cfg.Table)
	q := `INSERT INTO ` + tbl + ` (timestamp, target, source, subscription, name, path, tags, value_type, value_int, value_uint, value_float, value_bool, value_string, is_delete)`
	batch, err := conn.PrepareBatch(ctx, q)
	if err != nil {
		return err
	}
	defer batch.Close()
	for _, r := range rows {
		tags := r.tags
		if tags == nil {
			tags = map[string]string{}
		}
		if err := batch.Append(
			r.timestamp,
			r.target,
			r.source,
			r.subscription,
			r.name,
			r.path,
			tags,
			r.valueType,
			r.valueInt,
			r.valueUint,
			r.valueFloat,
			r.valueBool,
			r.valueString,
			r.isDelete,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (o *clickhouseOutput) Close() error {
	if o.cancelFn != nil {
		o.cancelFn()
	}
	if o.wg != nil {
		o.wg.Wait()
	}
	o.closeOnce.Do(func() {
		close(o.closeSig)
	})
	if p := o.conn.Load(); p != nil && *p != nil {
		_ = (*p).Close()
	}
	o.logger.Info("closed clickhouse output", slog.Any("config", o.String()))
	return nil
}
