// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package nats_input

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/inputs"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/pipeline"
	gutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
)

const (
	loggingPrefix           = "[nats_input] "
	natsReconnectBufferSize = 100 * 1024 * 1024
	defaultAddress          = "localhost:4222"
	natsConnectWait         = 2 * time.Second
	defaultFormat           = "event"
	defaultSubject          = "telemetry"
	defaultNumWorkers       = 1
	defaultBufferSize       = 100
)

func init() {
	inputs.Register("nats", func() inputs.Input {
		return &natsInput{
			confLock: new(sync.RWMutex),
			cfg:      new(atomic.Pointer[config]),
			dynCfg:   new(atomic.Pointer[dynConfig]),
			logger:   log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
			wg:       new(sync.WaitGroup),
		}
	})
}

// natsInput //
type natsInput struct {
	// ensure only one Update or UpdateProcessor operation
	// are performed at a time
	confLock *sync.RWMutex

	inputs.BaseInput
	cfg    *atomic.Pointer[config]
	dynCfg *atomic.Pointer[dynConfig]

	ctx    context.Context
	cfn    context.CancelFunc
	logger *log.Logger

	wg       *sync.WaitGroup
	outputs  []outputs.Output // used when the cmd is subscribe
	store    store.Store[any]
	pipeline chan *pipeline.Msg
}

type dynConfig struct {
	evps       []formatters.EventProcessor
	outputsMap map[string]struct{} // used when the cmd is collector
}

// config //
type config struct {
	Name            string           `mapstructure:"name,omitempty"`
	Address         string           `mapstructure:"address,omitempty"`
	Subject         string           `mapstructure:"subject,omitempty"`
	Queue           string           `mapstructure:"queue,omitempty"`
	Username        string           `mapstructure:"username,omitempty"`
	Password        string           `mapstructure:"password,omitempty"`
	ConnectTimeWait time.Duration    `mapstructure:"connect-time-wait,omitempty"`
	TLS             *types.TLSConfig `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	Format          string           `mapstructure:"format,omitempty"`
	Debug           bool             `mapstructure:"debug,omitempty"`
	NumWorkers      int              `mapstructure:"num-workers,omitempty"`
	BufferSize      int              `mapstructure:"buffer-size,omitempty"`
	Outputs         []string         `mapstructure:"outputs,omitempty"`
	EventProcessors []string         `mapstructure:"event-processors,omitempty"`
}

// Init //
func (n *natsInput) Start(ctx context.Context, name string, cfg map[string]any, opts ...inputs.Option) error {
	n.confLock.Lock()
	defer n.confLock.Unlock()

	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	if newCfg.Name == "" {
		newCfg.Name = name
	}
	n.logger.SetPrefix(fmt.Sprintf("%s%s", loggingPrefix, newCfg.Name))
	options := &inputs.InputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}
	n.store = options.Store
	n.pipeline = options.Pipeline

	n.setName(options.Name, newCfg)
	n.setLogger(options.Logger)
	outputs, outputsMap := n.getOutputs(options.Outputs, newCfg)
	n.outputs = outputs
	evps, err := n.buildEventProcessors(options.Logger, newCfg.EventProcessors)
	if err != nil {
		return err
	}
	err = n.setDefaultsFor(newCfg)
	if err != nil {
		return err
	}

	n.cfg.Store(newCfg)

	dc := &dynConfig{
		evps:       evps,
		outputsMap: outputsMap,
	}

	n.dynCfg.Store(dc)
	n.ctx = ctx                // save context for worker restarts
	var runCtx context.Context // create a run context for the workers
	runCtx, n.cfn = context.WithCancel(ctx)
	n.logger.Printf("input starting with config: %+v", newCfg)
	n.wg.Add(newCfg.NumWorkers)
	for i := 0; i < newCfg.NumWorkers; i++ {
		go n.worker(runCtx, i)
	}
	return nil
}

func (n *natsInput) Validate(cfg map[string]any) error {
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	return n.setDefaultsFor(newCfg)
}

// Update updates the input configuration and restarts the workers if
// necessary.
// It works only when the command is collector (not subscribe).
func (n *natsInput) Update(cfg map[string]any) error {
	n.confLock.Lock()
	defer n.confLock.Unlock()

	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	n.setDefaultsFor(newCfg)
	currCfg := n.cfg.Load()

	restartWorkers := needsWorkerRestart(currCfg, newCfg)
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0
	// build new dynamic config
	dc := &dynConfig{
		outputsMap: make(map[string]struct{}),
	}
	for _, o := range newCfg.Outputs {
		dc.outputsMap[o] = struct{}{}
	}

	prevDC := n.dynCfg.Load()

	if rebuildProcessors {
		dc.evps, err = n.buildEventProcessors(n.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}

	n.dynCfg.Store(dc)
	n.cfg.Store(newCfg)

	if restartWorkers {
		runCtx, cancel := context.WithCancel(n.ctx)
		newWG := new(sync.WaitGroup)
		// save old pointers
		oldCancel := n.cfn
		oldWG := n.wg
		// swap
		n.cfn = cancel
		n.wg = newWG

		n.wg.Add(newCfg.NumWorkers)
		for i := 0; i < newCfg.NumWorkers; i++ {
			go n.worker(runCtx, i)
		}
		// cancel old workers and loops
		if oldCancel != nil {
			oldCancel()
		}
		if oldWG != nil {
			oldWG.Wait()
		}
	}
	return nil
}

func (n *natsInput) UpdateProcessor(name string, pcfg map[string]any) error {
	n.confLock.Lock()
	defer n.confLock.Unlock()

	cfg := n.cfg.Load()
	dc := n.dynCfg.Load()

	newEvps, changed, err := inputs.UpdateProcessorInSlice(
		n.logger,
		n.store,
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
		n.dynCfg.Store(&newDC)
		n.logger.Printf("updated event processor %s", name)
	}
	return nil
}

func needsWorkerRestart(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.NumWorkers != nw.NumWorkers ||
		old.BufferSize != nw.BufferSize ||
		old.Address != nw.Address ||
		old.Subject != nw.Subject ||
		old.Queue != nw.Queue ||
		old.Username != nw.Username ||
		old.Password != nw.Password ||
		!old.TLS.Equal(nw.TLS) ||
		old.ConnectTimeWait != nw.ConnectTimeWait
}

func (n *natsInput) worker(ctx context.Context, idx int) {
	defer n.wg.Done()

	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	n.logger.Printf("%s starting", workerLogPrefix)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n.logger.Printf("worker %d loading config", idx)
		cfg := n.cfg.Load()
		wCfg := *cfg
		wCfg.Name = fmt.Sprintf("%s-%d", wCfg.Name, idx)
		fmt.Printf("worker %d starting with config: %+v", idx, wCfg)
		// scoped connection, subscription and cleanup
		err := n.doWork(ctx, &wCfg, workerLogPrefix)
		if err != nil {
			n.logger.Printf("%s NATS client failed: %v", workerLogPrefix, err)
		}

		// backoff before retry
		select {
		case <-ctx.Done():
			return
		case <-time.After(wCfg.ConnectTimeWait):
		}
	}
}

// scoped connection, subscription and cleanup
func (n *natsInput) doWork(ctx context.Context, wCfg *config, workerLogPrefix string) error {
	nc, err := n.createNATSConn(wCfg)
	if err != nil {
		return fmt.Errorf("create NATS connection: %w", err)
	}
	defer nc.Close()

	msgChan := make(chan *nats.Msg, wCfg.BufferSize)

	sub, err := nc.ChanQueueSubscribe(wCfg.Subject, wCfg.Queue, msgChan)
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return nil
		case m, ok := <-msgChan:
			if !ok {
				return fmt.Errorf("msg channel closed")
			}
			if len(m.Data) == 0 {
				continue
			}
			// load current config for dynamic fields like Format
			cfg := n.cfg.Load()
			if cfg.Debug {
				n.logger.Printf("received msg, subject=%s, queue=%s, len=%d, data=%s",
					m.Subject, m.Sub.Queue, len(m.Data), string(m.Data))
			}

			dc := n.dynCfg.Load()
			switch cfg.Format {
			case "event":
				var evMsgs []*formatters.EventMsg
				if err := json.Unmarshal(m.Data, &evMsgs); err != nil {
					if cfg.Debug {
						n.logger.Printf("%s failed to unmarshal event msg: %v", workerLogPrefix, err)
					}
					continue
				}
				for _, p := range dc.evps {
					evMsgs = p.Apply(evMsgs...)
				}

				if n.pipeline != nil {
					select {
					case <-ctx.Done():
						return nil
					case n.pipeline <- &pipeline.Msg{
						Events:  evMsgs,
						Outputs: dc.outputsMap,
					}:
					default:
						n.logger.Printf("pipeline channel is full, dropping event")
					}
				}
				for _, o := range n.outputs {
					for _, ev := range evMsgs {
						o.WriteEvent(ctx, ev)
					}
				}

			case "proto":
				protoMsg := new(gnmi.SubscribeResponse)
				if err := proto.Unmarshal(m.Data, protoMsg); err != nil {
					n.logger.Printf("failed to unmarshal proto msg: %v", err)
					continue
				}
				meta := outputs.Meta{}
				parts := strings.SplitN(m.Subject, ".", 3)
				if len(parts) == 3 {
					meta["source"] = strings.ReplaceAll(parts[1], "-", ".")
					meta["subscription-name"] = parts[2]
				}

				if n.pipeline != nil {
					select {
					case <-ctx.Done():
						return nil
					case n.pipeline <- &pipeline.Msg{
						Msg:     protoMsg,
						Meta:    meta,
						Outputs: dc.outputsMap,
					}:
					default:
						n.logger.Printf("pipeline channel is full, dropping message")
					}
				}
				for _, o := range n.outputs {
					o.Write(ctx, protoMsg, meta)
				}
			}
		}
	}
}

// Close //
func (n *natsInput) Close() error {
	if n.cfn != nil {
		n.cfn()
	}
	if n.wg != nil {
		n.wg.Wait()
	}
	return nil
}

// SetLogger //
func (n *natsInput) setLogger(logger *log.Logger) {
	if logger != nil && n.logger != nil {
		n.logger.SetOutput(logger.Writer())
		n.logger.SetFlags(logger.Flags())
	}
}

// SetOutputs //
func (n *natsInput) getOutputs(outs map[string]outputs.Output, cfg *config) ([]outputs.Output, map[string]struct{}) {
	outputs := make([]outputs.Output, 0)

	if len(cfg.Outputs) == 0 {
		for _, o := range outs {
			outputs = append(outputs, o)
		}
		return outputs, nil
	}
	outputsMap := make(map[string]struct{})
	for _, name := range cfg.Outputs {
		outputsMap[name] = struct{}{} // for collector
		if o, ok := outs[name]; ok {  // for subscribe
			outputs = append(outputs, o)
		}
	}
	return outputs, outputsMap
}

func (n *natsInput) setName(name string, cfg *config) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(cfg.Name)
	sb.WriteString("-nats-sub")
	cfg.Name = sb.String()
}

func (n *natsInput) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(n.store)
	if err != nil {
		return nil, err
	}
	return formatters.MakeEventProcessors(
		logger,
		eventProcessors,
		ps,
		tcs,
		acts,
	)
}

// helper functions

func (n *natsInput) setDefaultsFor(cfg *config) error {
	if cfg.Format == "" {
		cfg.Format = defaultFormat
	}
	if !(strings.ToLower(cfg.Format) == "event" || strings.ToLower(cfg.Format) == "proto") {
		return fmt.Errorf("unsupported input format")
	}
	cfg.Format = strings.ToLower(cfg.Format)
	if cfg.Name == "" {
		cfg.Name = "gnmic-" + uuid.New().String()
	}
	if cfg.Subject == "" {
		cfg.Subject = defaultSubject
	}
	if cfg.Address == "" {
		cfg.Address = defaultAddress
	}
	if cfg.ConnectTimeWait <= 0 {
		cfg.ConnectTimeWait = natsConnectWait
	}
	if cfg.Queue == "" {
		cfg.Queue = cfg.Name
	}
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = defaultNumWorkers
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = defaultBufferSize
	}
	return nil
}

func (n *natsInput) createNATSConn(c *config) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(c.Name),
		nats.SetCustomDialer(n),
		nats.ReconnectWait(c.ConnectTimeWait),
		nats.ReconnectBufSize(natsReconnectBufferSize),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			n.logger.Printf("NATS error: %v", err)
		}),
		nats.DisconnectHandler(func(*nats.Conn) {
			n.logger.Println("Disconnected from NATS")
		}),
		nats.ClosedHandler(func(*nats.Conn) {
			n.logger.Println("NATS connection is closed")
		}),
	}
	if c.Username != "" && c.Password != "" {
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	}
	if c.TLS != nil {
		tlsConfig, err := utils.NewTLSConfig(
			c.TLS.CaFile, c.TLS.CertFile, c.TLS.KeyFile, "", c.TLS.SkipVerify,
			false)
		if err != nil {
			return nil, err
		}
		if tlsConfig != nil {
			opts = append(opts, nats.Secure(tlsConfig))
		}
	}
	nc, err := nats.Connect(c.Address, opts...)
	if err != nil {
		return nil, err
	}
	return nc, nil
}

// Dial //
func (n *natsInput) Dial(network, address string) (net.Conn, error) {
	ctx, cancel := context.WithCancel(n.ctx)
	defer cancel()

	for {
		n.logger.Printf("attempting to connect to %s", address)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		cfg := n.cfg.Load()
		select {
		case <-n.ctx.Done():
			return nil, n.ctx.Err()
		default:
			d := &net.Dialer{}
			if conn, err := d.DialContext(ctx, network, address); err == nil {
				n.logger.Printf("successfully connected to NATS server %s", address)
				return conn, nil
			}
			time.Sleep(cfg.ConnectTimeWait)
		}
	}
}
