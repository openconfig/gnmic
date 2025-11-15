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
	"log"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/store"
	gutils "github.com/openconfig/gnmic/pkg/utils"
)

const (
	defaultRetryTimer = 2 * time.Second
	defaultNumWorkers = 1
	loggingPrefix     = "[tcp_output:%s] "
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
	logger   *log.Logger

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
	NumWorkers         int           `mapstructure:"num-workers,omitempty"`
	EnableMetrics      bool          `mapstructure:"enable-metrics,omitempty"`
	EventProcessors    []string      `mapstructure:"event-processors,omitempty"`
}

func (t *tcpOutput) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
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
	t.logger = log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags)
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
	t.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	t.store = options.Store

	// apply logger
	if options.Logger != nil && t.logger != nil {
		t.logger.SetOutput(options.Logger.Writer())
		t.logger.SetFlags(options.Logger.Flags())
	}

	dc := new(dynConfig)
	// initialize event processors
	dc.evps, err = t.buildEventProcessors(options.Logger, newCfg.EventProcessors)
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
		go t.start(ctx, i)
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
}

func validate(cfg *config) error {
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
	oldChan := *t.buffer.Load()
	oldWg := t.wg
	t.wg = new(sync.WaitGroup)
	oldCancel := t.cancelFn
	if swapChannel || restartWorkers {
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
		t.wg.Add(newCfg.NumWorkers)
		for i := 0; i < newCfg.NumWorkers; i++ {
			go t.start(ctx, i)
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
		t.logger.Printf("restarted TCP output workers")
	} else {
		t.logger.Printf("no changes to TCP output")
	}
	t.logger.Printf("updated TCP output: %s", t.String())
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
		t.logger.Printf("updated event processor %s", name)
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
			t.logger.Printf("failed to add target to the response: %v", err)
		}
		bb, err := outputs.Marshal(rsp, meta, dc.mo, cfg.SplitEvents, dc.evps...)
		if err != nil {
			t.logger.Printf("failed marshaling proto msg: %v", err)
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

func (t *tcpOutput) start(ctx context.Context, idx int) {
	defer t.wg.Done()
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
START:
	if ctx.Err() != nil {
		t.logger.Printf("context error: %v", ctx.Err())
		return
	}
	cfg := t.cfg.Load()
	dc := t.dynCfg.Load()
	tcpAddr, err := net.ResolveTCPAddr("tcp", cfg.Address)
	if err != nil {
		t.logger.Printf("%s failed to resolve address: %v", workerLogPrefix, err)
		time.Sleep(cfg.RetryInterval)
		goto START
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		t.logger.Printf("%s failed to dial TCP: %v", workerLogPrefix, err)
		time.Sleep(cfg.RetryInterval)
		goto START
	}
	defer conn.Close()
	if cfg.KeepAlive > 0 {
		conn.SetKeepAlive(true)
		conn.SetKeepAlivePeriod(cfg.KeepAlive)
	}
	buffer := *t.buffer.Load()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-buffer:
			delimiter := dc.delimiter
			if dc.limiter != nil {
				<-dc.limiter.C
			}
			// append delimiter
			b = append(b, delimiter...)
			_, err = conn.Write(b)
			if err != nil {
				t.logger.Printf("%s failed sending tcp bytes: %v", workerLogPrefix, err)
				conn.Close()
				time.Sleep(cfg.RetryInterval)
				goto START
			}
		}
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
