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
	"fmt"
	"io"
	"log"
	"net"
	"text/template"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	gutils "github.com/openconfig/gnmic/pkg/utils"
)

const (
	defaultRetryTimer = 2 * time.Second
	defaultNumWorkers = 1
	loggingPrefix     = "[tcp_output:%s] "
)

func init() {
	outputs.Register("tcp", func() outputs.Output {
		return &tcpOutput{
			cfg:    &config{},
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type tcpOutput struct {
	outputs.BaseOutput
	cfg *config

	cancelFn context.CancelFunc
	buffer   chan []byte
	limiter  *time.Ticker
	logger   *log.Logger
	mo       *formatters.MarshalOptions
	evps     []formatters.EventProcessor

	targetTpl *template.Template
	delimiter []byte
	store     store.Store[any]
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

func (t *tcpOutput) setEventProcessors(logger *log.Logger) error {
	tcs, ps, acts, err := gutils.GetConfigMaps(t.store)
	if err != nil {
		return err
	}
	t.evps, err = formatters.MakeEventProcessors(
		logger,
		t.cfg.EventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return err
	}
	return nil
}

func (t *tcpOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, t.cfg)
	if err != nil {
		return err
	}
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

	// initialize event processors
	err = t.setEventProcessors(options.Logger)
	if err != nil {
		return err
	}
	_, _, err = net.SplitHostPort(t.cfg.Address)
	if err != nil {
		return fmt.Errorf("wrong address format: %v", err)
	}
	t.buffer = make(chan []byte, t.cfg.BufferSize)
	if t.cfg.Rate > 0 {
		t.limiter = time.NewTicker(t.cfg.Rate)
	}
	if t.cfg.RetryInterval == 0 {
		t.cfg.RetryInterval = defaultRetryTimer
	}
	if t.cfg.NumWorkers < 1 {
		t.cfg.NumWorkers = defaultNumWorkers
	}
	if len(t.cfg.Delimiter) > 0 {
		t.delimiter = []byte(t.cfg.Delimiter)
	}
	t.mo = &formatters.MarshalOptions{
		Format:     t.cfg.Format,
		OverrideTS: t.cfg.OverrideTimestamps,
	}

	if t.cfg.TargetTemplate == "" {
		t.targetTpl = outputs.DefaultTargetTemplate
	} else if t.cfg.AddTarget != "" {
		t.targetTpl, err = gtemplate.CreateTemplate("target-template", t.cfg.TargetTemplate)
		if err != nil {
			return err
		}
		t.targetTpl = t.targetTpl.Funcs(outputs.TemplateFuncs)
	}
	go func() {
		<-ctx.Done()
		t.Close()
	}()

	ctx, t.cancelFn = context.WithCancel(ctx)
	for i := 0; i < t.cfg.NumWorkers; i++ {
		go t.start(ctx, i)
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
		rsp, err := outputs.AddSubscriptionTarget(m, meta, t.cfg.AddTarget, t.targetTpl)
		if err != nil {
			t.logger.Printf("failed to add target to the response: %v", err)
		}
		bb, err := outputs.Marshal(rsp, meta, t.mo, t.cfg.SplitEvents, t.evps...)
		if err != nil {
			t.logger.Printf("failed marshaling proto msg: %v", err)
			return
		}
		for _, b := range bb {
			t.buffer <- b
		}
	}
}

func (t *tcpOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

func (t *tcpOutput) Close() error {
	t.cancelFn()
	if t.limiter != nil {
		t.limiter.Stop()
	}
	return nil
}

func (t *tcpOutput) String() string {
	b, err := json.Marshal(t.cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (t *tcpOutput) start(ctx context.Context, idx int) {
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
START:
	tcpAddr, err := net.ResolveTCPAddr("tcp", t.cfg.Address)
	if err != nil {
		t.logger.Printf("%s failed to resolve address: %v", workerLogPrefix, err)
		time.Sleep(t.cfg.RetryInterval)
		goto START
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		t.logger.Printf("%s failed to dial TCP: %v", workerLogPrefix, err)
		time.Sleep(t.cfg.RetryInterval)
		goto START
	}
	defer conn.Close()
	if t.cfg.KeepAlive > 0 {
		conn.SetKeepAlive(true)
		conn.SetKeepAlivePeriod(t.cfg.KeepAlive)
	}
	defer t.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-t.buffer:
			if t.limiter != nil {
				<-t.limiter.C
			}
			// append delimiter
			b = append(b, t.delimiter...)
			_, err = conn.Write(b)
			if err != nil {
				t.logger.Printf("%s failed sending tcp bytes: %v", workerLogPrefix, err)
				conn.Close()
				time.Sleep(t.cfg.RetryInterval)
				goto START
			}
		}
	}
}
