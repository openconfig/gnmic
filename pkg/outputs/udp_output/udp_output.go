// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package udp_output

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
	loggingPrefix     = "[udp_output:%s] "
)

func init() {
	outputs.Register("udp", func() outputs.Output {
		return &udpSock{}
	})
}

type udpSock struct {
	outputs.BaseOutput

	cfg    *atomic.Pointer[Config]
	dynCfg *atomic.Pointer[dynConfig]
	buffer *atomic.Pointer[chan []byte]

	rootCtx  context.Context
	cancelFn context.CancelFunc
	wg       *sync.WaitGroup
	logger   *log.Logger

	store store.Store[any]
}

type dynConfig struct {
	targetTpl *template.Template
	evps      []formatters.EventProcessor
	mo        *formatters.MarshalOptions
	limiter   *time.Ticker
}

type Config struct {
	Address            string        `mapstructure:"address,omitempty"` // ip:port
	Rate               time.Duration `mapstructure:"rate,omitempty"`
	BufferSize         uint          `mapstructure:"buffer-size,omitempty"`
	Format             string        `mapstructure:"format,omitempty"`
	AddTarget          string        `mapstructure:"add-target,omitempty"`
	TargetTemplate     string        `mapstructure:"target-template,omitempty"`
	OverrideTimestamps bool          `mapstructure:"override-timestamps,omitempty"`
	SplitEvents        bool          `mapstructure:"split-events,omitempty"`
	RetryInterval      time.Duration `mapstructure:"retry-interval,omitempty"`
	EnableMetrics      bool          `mapstructure:"enable-metrics,omitempty"`
	EventProcessors    []string      `mapstructure:"event-processors,omitempty"`
}

func (u *udpSock) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(u.store)
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

func (u *udpSock) init() {
	u.cfg = new(atomic.Pointer[Config])
	u.dynCfg = new(atomic.Pointer[dynConfig])
	u.buffer = new(atomic.Pointer[chan []byte])
	u.wg = new(sync.WaitGroup)
	u.logger = log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags)
}

func (u *udpSock) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	u.init()

	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	setDefaultsFor(newCfg)

	u.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	u.store = options.Store

	// apply logger
	if options.Logger != nil && u.logger != nil {
		u.logger.SetOutput(options.Logger.Writer())
		u.logger.SetFlags(options.Logger.Flags())
	}

	dc := new(dynConfig)
	// initialize event processors
	dc.evps, err = u.buildEventProcessors(options.Logger, newCfg.EventProcessors)
	if err != nil {
		return err
	}

	_, _, err = net.SplitHostPort(newCfg.Address)
	if err != nil {
		return fmt.Errorf("wrong address format: %v", err)
	}

	if newCfg.Rate > 0 {
		dc.limiter = time.NewTicker(newCfg.Rate)
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

	ch := make(chan []byte, newCfg.BufferSize)
	u.buffer.Store(&ch)
	u.dynCfg.Store(dc)
	u.cfg.Store(newCfg)

	u.rootCtx = ctx
	u.rootCtx, u.cancelFn = context.WithCancel(u.rootCtx)

	u.wg.Add(1)
	go u.start(u.rootCtx)

	u.logger.Printf("initialized UDP output: %s", u.String())
	return nil
}

func setDefaultsFor(cfg *Config) {
	if cfg.RetryInterval == 0 {
		cfg.RetryInterval = defaultRetryTimer
	}
}

func validate(cfg *Config) error {
	if cfg.Address == "" {
		return errors.New("address is required")
	}
	_, _, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return fmt.Errorf("wrong address format: %v", err)
	}
	return nil
}

func (u *udpSock) Validate(cfg map[string]any) error {
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	setDefaultsFor(newCfg)
	return validate(newCfg)
}

func (u *udpSock) Update(_ context.Context, cfg map[string]any) error {
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	setDefaultsFor(newCfg)
	currCfg := u.cfg.Load()

	swapChannel := channelNeedsSwap(currCfg, newCfg)
	restartWorker := needsWorkerRestart(currCfg, newCfg)
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0

	dc := new(dynConfig)
	prevDC := u.dynCfg.Load()

	if rebuildProcessors {
		dc.evps, err = u.buildEventProcessors(u.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}

	// handle rate limiter changes
	if newCfg.Rate > 0 {
		if currCfg.Rate != newCfg.Rate {
			// rate changed, stop old limiter and create new one
			if prevDC != nil && prevDC.limiter != nil {
				prevDC.limiter.Stop()
			}
			dc.limiter = time.NewTicker(newCfg.Rate)
		} else {
			// rate unchanged, copy old limiter
			if prevDC != nil {
				dc.limiter = prevDC.limiter
			}
		}
	} else {
		// no rate limiting, stop old limiter if any
		if prevDC != nil && prevDC.limiter != nil {
			prevDC.limiter.Stop()
		}
		dc.limiter = nil
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

	// store new configs
	u.dynCfg.Store(dc)
	u.cfg.Store(newCfg)

	if swapChannel || restartWorker {
		var newChan chan []byte
		oldChan := *u.buffer.Load()

		if swapChannel {
			newChan = make(chan []byte, newCfg.BufferSize)
		} else {
			newChan = oldChan
		}

		// create new context and WaitGroup
		runCtx, cancel := context.WithCancel(u.rootCtx)
		newWG := new(sync.WaitGroup)

		// Save old pointers
		oldCancel := u.cancelFn
		oldWG := u.wg

		// swap
		u.cancelFn = cancel
		u.wg = newWG
		u.buffer.Store(&newChan)

		// start new worker
		u.wg.Add(1)
		go u.start(runCtx)

		// cancel old worker and wait
		if oldCancel != nil {
			oldCancel()
		}
		if oldWG != nil {
			oldWG.Wait()
		}

		// drain old channel if we swapped
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
						// new channel is full, drop message
					}
				default:
					break DRAIN_LOOP
				}
			}
		}

		u.logger.Printf("restarted UDP output worker")
	}

	u.logger.Printf("updated UDP output: %s", u.String())
	return nil
}

func (u *udpSock) UpdateProcessor(name string, pcfg map[string]any) error {
	cfg := u.cfg.Load()
	dc := u.dynCfg.Load()

	newEvps, changed, err := outputs.UpdateProcessorInSlice(
		u.logger,
		u.store,
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
		u.dynCfg.Store(&newDC)
		u.logger.Printf("updated event processor %s", name)
	}
	return nil
}

func (u *udpSock) Write(ctx context.Context, m proto.Message, meta outputs.Meta) {
	if m == nil {
		return
	}

	select {
	case <-ctx.Done():
		return
	default:
		cfg := u.cfg.Load()
		dc := u.dynCfg.Load()
		if cfg == nil || dc == nil {
			return
		}

		rsp, err := outputs.AddSubscriptionTarget(m, meta, cfg.AddTarget, dc.targetTpl)
		if err != nil {
			u.logger.Printf("failed to add target to the response: %v", err)
		}
		bb, err := outputs.Marshal(rsp, meta, dc.mo, cfg.SplitEvents, dc.evps...)
		if err != nil {
			u.logger.Printf("failed marshaling proto msg: %v", err)
			return
		}
		buffer := u.buffer.Load()
		if buffer == nil {
			return
		}
		for _, b := range bb {
			select {
			case <-ctx.Done():
				return
			case (*buffer) <- b:
			}
		}
	}
}

func (u *udpSock) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

func (u *udpSock) Close() error {
	if u.cancelFn != nil {
		u.cancelFn()
	}
	u.wg.Wait()

	// Stop limiter if exists
	dc := u.dynCfg.Load()
	if dc != nil && dc.limiter != nil {
		dc.limiter.Stop()
	}
	return nil
}

func (u *udpSock) String() string {
	cfg := u.cfg.Load()
	if cfg == nil {
		return ""
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (u *udpSock) start(ctx context.Context) {
	defer u.wg.Done()

DIAL:
	if ctx.Err() != nil {
		u.logger.Printf("context error: %v", ctx.Err())
		return
	}

	cfg := u.cfg.Load()
	dc := u.dynCfg.Load()
	if cfg == nil || dc == nil {
		u.logger.Printf("config not loaded")
		return
	}

	udpAddr, err := net.ResolveUDPAddr("udp", cfg.Address)
	if err != nil {
		u.logger.Printf("failed to resolve UDP address: %v", err)
		time.Sleep(cfg.RetryInterval)
		goto DIAL
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		u.logger.Printf("failed to dial UDP: %v", err)
		time.Sleep(cfg.RetryInterval)
		goto DIAL
	}
	defer conn.Close()

	u.logger.Printf("connected to %s", cfg.Address)

	// Snapshot buffer and limiter at connection time
	buffer := *u.buffer.Load()
	limiter := dc.limiter

	for {
		select {
		case <-ctx.Done():
			u.logger.Printf("UDP worker shutting down")
			return
		case b, ok := <-buffer:
			if !ok {
				u.logger.Printf("buffer channel closed")
				return
			}

			if limiter != nil {
				select {
				case <-ctx.Done():
					return
				case <-limiter.C:
				}
			}

			_, err = conn.Write(b)
			if err != nil {
				u.logger.Printf("failed sending UDP bytes: %v", err)
				conn.Close()
				time.Sleep(cfg.RetryInterval)
				goto DIAL
			}
		}
	}
}

func channelNeedsSwap(old, nw *Config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.BufferSize != nw.BufferSize
}

func needsWorkerRestart(old, nw *Config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.Address != nw.Address
}
