// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"text/template"
	"time"

	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/proto"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
)

const (
	defaultFormat           = "json"
	defaultWriteConcurrency = 1000
	defaultSeparator        = "\n"
	loggingPrefix           = "[file_output:%s] "
)

func init() {
	outputs.Register("file", func() outputs.Output {
		return &File{
			cfg:    &Config{},
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// File //
type File struct {
	cfg    *Config
	file   file
	logger *log.Logger
	mo     *formatters.MarshalOptions
	sem    *semaphore.Weighted
	evps   []formatters.EventProcessor

	targetTpl *template.Template
	msgTpl    *template.Template
}

// Config //
type Config struct {
	FileName           string          `mapstructure:"filename,omitempty"`
	FileType           string          `mapstructure:"file-type,omitempty"`
	Format             string          `mapstructure:"format,omitempty"`
	Multiline          bool            `mapstructure:"multiline,omitempty"`
	Indent             string          `mapstructure:"indent,omitempty"`
	Separator          string          `mapstructure:"separator,omitempty"`
	SplitEvents        bool            `mapstructure:"split-events,omitempty"`
	OverrideTimestamps bool            `mapstructure:"override-timestamps,omitempty"`
	AddTarget          string          `mapstructure:"add-target,omitempty"`
	TargetTemplate     string          `mapstructure:"target-template,omitempty"`
	EventProcessors    []string        `mapstructure:"event-processors,omitempty"`
	MsgTemplate        string          `mapstructure:"msg-template,omitempty"`
	ConcurrencyLimit   int             `mapstructure:"concurrency-limit,omitempty"`
	EnableMetrics      bool            `mapstructure:"enable-metrics,omitempty"`
	Debug              bool            `mapstructure:"debug,omitempty"`
	CalculateLatency   bool            `mapstructure:"calculate-latency,omitempty"`
	Rotation           *rotationConfig `mapstructure:"rotation,omitempty"`
}

type file interface {
	Close() error
	Name() string
	Write([]byte) (int, error)
}

func (f *File) String() string {
	b, err := json.Marshal(f.cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (f *File) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) error {
	var err error
	f.evps, err = formatters.MakeEventProcessors(
		logger,
		f.cfg.EventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return err
	}
	return nil
}

func (f *File) SetLogger(logger *log.Logger) {
	if logger != nil && f.logger != nil {
		f.logger.SetOutput(logger.Writer())
		f.logger.SetFlags(logger.Flags())
	}
}

// Init //
func (f *File) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, f.cfg)
	if err != nil {
		return err
	}

	f.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))

	for _, opt := range opts {
		if err := opt(f); err != nil {
			return err
		}
	}
	if f.cfg.Format == "proto" {
		return fmt.Errorf("proto format not supported in output type 'file'")
	}
	if f.cfg.Separator == "" {
		f.cfg.Separator = defaultSeparator
	}
	if f.cfg.FileName == "" && f.cfg.FileType == "" {
		f.cfg.FileType = "stdout"
	}

	switch f.cfg.FileType {
	case "stdout":
		f.file = os.Stdout
	case "stderr":
		f.file = os.Stderr
	default:
	CRFILE:
		if f.cfg.Rotation != nil {
			f.file = newRotatingFile(f.cfg)
		} else {
			f.file, err = os.OpenFile(f.cfg.FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				f.logger.Printf("failed to create file: %v", err)
				time.Sleep(10 * time.Second)
				goto CRFILE
			}
		}
	}

	if f.cfg.Format == "" {
		f.cfg.Format = defaultFormat
	}
	if f.cfg.FileType == "stdout" || f.cfg.FileType == "stderr" {
		f.cfg.Indent = "  "
		f.cfg.Multiline = true
	}
	if f.cfg.Multiline && f.cfg.Indent == "" {
		f.cfg.Indent = "  "
	}
	if f.cfg.ConcurrencyLimit < 1 {
		switch f.cfg.FileType {
		case "stdout", "stderr":
			f.cfg.ConcurrencyLimit = 1
		default:
			f.cfg.ConcurrencyLimit = defaultWriteConcurrency
		}
	}

	f.sem = semaphore.NewWeighted(int64(f.cfg.ConcurrencyLimit))

	f.mo = &formatters.MarshalOptions{
		Multiline:        f.cfg.Multiline,
		Indent:           f.cfg.Indent,
		Format:           f.cfg.Format,
		OverrideTS:       f.cfg.OverrideTimestamps,
		CalculateLatency: f.cfg.CalculateLatency,
	}
	if f.cfg.TargetTemplate == "" {
		f.targetTpl = outputs.DefaultTargetTemplate
	} else if f.cfg.AddTarget != "" {
		f.targetTpl, err = gtemplate.CreateTemplate("target-template", f.cfg.TargetTemplate)
		if err != nil {
			return err
		}
		f.targetTpl = f.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	if f.cfg.MsgTemplate != "" {
		f.msgTpl, err = gtemplate.CreateTemplate(fmt.Sprintf("%s-msg-template", name), f.cfg.MsgTemplate)
		if err != nil {
			return err
		}
		f.msgTpl = f.msgTpl.Funcs(outputs.TemplateFuncs)
	}

	f.logger.Printf("initialized file output: %s", f.String())
	go func() {
		<-ctx.Done()
		f.Close()
	}()
	return nil
}

// Write //
func (f *File) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	err := f.sem.Acquire(ctx, 1)
	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		f.logger.Printf("failed acquiring semaphore: %v", err)
		return
	}
	defer f.sem.Release(1)

	numberOfReceivedMsgs.WithLabelValues(f.file.Name()).Inc()
	rsp, err = outputs.AddSubscriptionTarget(rsp, meta, f.cfg.AddTarget, f.targetTpl)
	if err != nil {
		f.logger.Printf("failed to add target to the response: %v", err)
	}
	bb, err := outputs.Marshal(rsp, meta, f.mo, f.cfg.SplitEvents, f.evps...)
	if err != nil {
		if f.cfg.Debug {
			f.logger.Printf("failed marshaling proto msg: %v", err)
		}
		numberOfFailWriteMsgs.WithLabelValues(f.file.Name(), "marshal_error").Inc()
		return
	}
	if len(bb) == 0 {
		return
	}
	for _, b := range bb {
		if f.msgTpl != nil {
			b, err = outputs.ExecTemplate(b, f.msgTpl)
			if err != nil {
				if f.cfg.Debug {
					log.Printf("failed to execute template: %v", err)
				}
				numberOfFailWriteMsgs.WithLabelValues(f.file.Name(), "template_error").Inc()
				continue
			}
		}

		n, err := f.file.Write(append(b, []byte(f.cfg.Separator)...))
		if err != nil {
			if f.cfg.Debug {
				f.logger.Printf("failed to write to file '%s': %v", f.file.Name(), err)
			}
			numberOfFailWriteMsgs.WithLabelValues(f.file.Name(), "write_error").Inc()
			return
		}
		numberOfWrittenBytes.WithLabelValues(f.file.Name()).Add(float64(n))
		numberOfWrittenMsgs.WithLabelValues(f.file.Name()).Inc()
	}
}

func (f *File) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	select {
	case <-ctx.Done():
		return
	default:
	}
	var evs = []*formatters.EventMsg{ev}
	for _, proc := range f.evps {
		evs = proc.Apply(evs...)
	}
	toWrite := []byte{}
	if f.cfg.SplitEvents {
		for _, pev := range evs {
			var err error
			var b []byte
			if f.cfg.Multiline {
				b, err = json.MarshalIndent(pev, "", f.cfg.Indent)
			} else {
				b, err = json.Marshal(pev)
			}
			if err != nil {
				fmt.Printf("failed to WriteEvent: %v", err)
				numberOfFailWriteMsgs.WithLabelValues(f.file.Name(), "marshal_error").Inc()
				return
			}
			toWrite = append(toWrite, b...)
			toWrite = append(toWrite, []byte(f.cfg.Separator)...)
		}
	} else {
		var err error
		var b []byte
		if f.cfg.Multiline {
			b, err = json.MarshalIndent(evs, "", f.cfg.Indent)
		} else {
			b, err = json.Marshal(evs)
		}
		if err != nil {
			fmt.Printf("failed to WriteEvent: %v", err)
			numberOfFailWriteMsgs.WithLabelValues(f.file.Name(), "marshal_error").Inc()
			return
		}
		toWrite = append(toWrite, b...)
		toWrite = append(toWrite, []byte(f.cfg.Separator)...)
	}

	n, err := f.file.Write(toWrite)
	if err != nil {
		fmt.Printf("failed to WriteEvent: %v", err)
		numberOfFailWriteMsgs.WithLabelValues(f.file.Name(), "write_error").Inc()
		return
	}
	numberOfWrittenBytes.WithLabelValues(f.file.Name()).Add(float64(n))
	numberOfWrittenMsgs.WithLabelValues(f.file.Name()).Inc()
}

// Close //
func (f *File) Close() error {
	f.logger.Printf("closing file '%s' output", f.file.Name())
	return f.file.Close()
}

// Metrics //
func (f *File) RegisterMetrics(reg *prometheus.Registry) {
	if !f.cfg.EnableMetrics {
		return
	}
	if err := registerMetrics(reg); err != nil {
		f.logger.Printf("failed to register metric: %v", err)
	}
}

func (f *File) SetName(name string)                             {}
func (f *File) SetClusterName(name string)                      {}
func (f *File) SetTargetsConfig(map[string]*types.TargetConfig) {}
