// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
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
	"slices"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	gutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
)

const (
	defaultFormat           = "json"
	defaultWriteConcurrency = 1000
	defaultSeparator        = "\n"
	loggingPrefix           = "[file_output:%s] "
)

const (
	outputType      = "file"
	fileType_STDOUT = "stdout"
	fileType_STDERR = "stderr"
)

func init() {
	outputs.Register(outputType, func() outputs.Output {
		return &File{}
	})
}

func (f *File) init() {
	f.cfg = new(atomic.Pointer[config])
	f.dynCfg = new(atomic.Pointer[dynConfig])
	f.file = new(atomic.Pointer[file])
	f.logger = log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags)
}

// File //
type File struct {
	outputs.BaseOutput

	cfg    *atomic.Pointer[config]
	dynCfg *atomic.Pointer[dynConfig]
	file   *atomic.Pointer[file]

	logger *log.Logger
	sem    *semaphore.Weighted

	reg   *prometheus.Registry
	store store.Store[any]
}

type dynConfig struct {
	targetTpl *template.Template
	msgTpl    *template.Template
	evps      []formatters.EventProcessor
	mo        *formatters.MarshalOptions
}

// config //
type config struct {
	Name               string          `mapstructure:"name,omitempty"`
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
	cfg := f.cfg.Load()
	if cfg == nil {
		return ""
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (f *File) setDefaults(cfg *config) error {
	if cfg.Format == "proto" {
		return fmt.Errorf("proto format not supported in output type 'file'")
	}
	if cfg.Separator == "" {
		cfg.Separator = defaultSeparator
	}
	if cfg.FileName == "" && cfg.FileType == "" {
		cfg.FileType = fileType_STDOUT
	}
	if cfg.Format == "" {
		cfg.Format = defaultFormat
	}
	if cfg.FileType == fileType_STDOUT || cfg.FileType == fileType_STDERR {
		cfg.Indent = "  "
		cfg.Multiline = true
	}
	if cfg.Multiline && cfg.Indent == "" {
		cfg.Indent = "  "
	}
	if cfg.ConcurrencyLimit < 1 {
		switch cfg.FileType {
		case fileType_STDOUT, fileType_STDERR:
			cfg.ConcurrencyLimit = 1
		default:
			cfg.ConcurrencyLimit = defaultWriteConcurrency
		}
	}
	return nil
}

// Init //
func (f *File) Init(ctx context.Context, name string, cfg map[string]any, opts ...outputs.Option) error {
	f.init()
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	if newCfg.Name == "" {
		newCfg.Name = name
	}
	f.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	f.store = options.Store

	// apply logger
	f.setLogger(options.Logger)

	err = f.setDefaults(newCfg)
	if err != nil {
		return err
	}

	// store config
	f.cfg.Store(newCfg)

	// initialize registry
	f.reg = options.Registry
	err = f.registerMetrics()
	if err != nil {
		return err
	}

	// initialize semaphore
	f.sem = semaphore.NewWeighted(int64(newCfg.ConcurrencyLimit))

	// build dynamic config
	dc := new(dynConfig)

	// initialize event processors
	dc.evps, err = f.buildEventProcessors(options.Logger, newCfg.EventProcessors)
	if err != nil {
		return err
	}

	dc.mo = &formatters.MarshalOptions{
		Multiline:        newCfg.Multiline,
		Indent:           newCfg.Indent,
		Format:           newCfg.Format,
		OverrideTS:       newCfg.OverrideTimestamps,
		CalculateLatency: newCfg.CalculateLatency,
	}

	// create templates
	if newCfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	if newCfg.MsgTemplate != "" {
		dc.msgTpl, err = gtemplate.CreateTemplate(fmt.Sprintf("%s-msg-template", name), newCfg.MsgTemplate)
		if err != nil {
			return err
		}
		dc.msgTpl = dc.msgTpl.Funcs(outputs.TemplateFuncs)
	}

	f.dynCfg.Store(dc)

	// initialize file
	newFile, err := f.openFile(newCfg)
	if err != nil {
		return err
	}
	f.file.Store(&newFile)

	f.logger.Printf("initialized file output: %s", f.String())
	return nil
}

func (f *File) Validate(cfg map[string]any) error {
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	if newCfg.Format == "proto" {
		return fmt.Errorf("proto format not supported in output type 'file'")
	}
	return nil
}

func (f *File) openFile(cfg *config) (file, error) {
	var fileHandle file
	var err error

	switch cfg.FileType {
	case fileType_STDOUT:
		return os.Stdout, nil
	case fileType_STDERR:
		return os.Stderr, nil
	default:
	CRFILE:
		if cfg.Rotation != nil {
			fileHandle = newRotatingFile(cfg)
		} else {
			fileHandle, err = os.OpenFile(cfg.FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				f.logger.Printf("failed to create file: %v", err)
				time.Sleep(10 * time.Second)
				goto CRFILE
			}
		}
	}
	return fileHandle, nil
}

func (f *File) Update(ctx context.Context, cfgMap map[string]any) error {
	newCfg := new(config)
	err := outputs.DecodeConfig(cfgMap, newCfg)
	if err != nil {
		return err
	}

	currCfg := f.cfg.Load()
	if newCfg.Name == "" && currCfg != nil {
		newCfg.Name = currCfg.Name
	}

	err = f.setDefaults(newCfg)
	if err != nil {
		return err
	}

	// check if we need to rebuild processors
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0

	// build new dynamic config
	dc := new(dynConfig)

	// rebuild processors if needed
	prevDC := f.dynCfg.Load()
	if rebuildProcessors {
		dc.evps, err = f.buildEventProcessors(f.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}

	dc.mo = &formatters.MarshalOptions{
		Multiline:        newCfg.Multiline,
		Indent:           newCfg.Indent,
		Format:           newCfg.Format,
		OverrideTS:       newCfg.OverrideTimestamps,
		CalculateLatency: newCfg.CalculateLatency,
	}

	// rebuild templates
	var targetTpl *template.Template
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
	dc.targetTpl = targetTpl

	if newCfg.MsgTemplate != "" {
		dc.msgTpl, err = gtemplate.CreateTemplate(fmt.Sprintf("%s-msg-template", newCfg.Name), newCfg.MsgTemplate)
		if err != nil {
			return err
		}
		dc.msgTpl = dc.msgTpl.Funcs(outputs.TemplateFuncs)
	}

	// store new dynamic config
	f.dynCfg.Store(dc)

	// check if file needs to be reopened
	needsFileReopen := fileNeedsReopen(currCfg, newCfg)

	if needsFileReopen {
		// open new file
		newFile, err := f.openFile(newCfg)
		if err != nil {
			return err
		}

		// swap file handle
		oldFile := f.file.Swap(&newFile)

		// close old file (but not stdout/stderr)
		if oldFile != nil && *oldFile != os.Stdout && *oldFile != os.Stderr {
			(*oldFile).Close()
		}
	}

	// update semaphore if concurrency limit changed
	if currCfg == nil || currCfg.ConcurrencyLimit != newCfg.ConcurrencyLimit {
		f.sem = semaphore.NewWeighted(int64(newCfg.ConcurrencyLimit))
	}

	// store new config
	f.cfg.Store(newCfg)

	f.logger.Printf("updated file output: %s", f.String())
	return nil
}

func (f *File) UpdateProcessor(name string, pcfg map[string]any) error {
	cfg := f.cfg.Load()
	dc := f.dynCfg.Load()

	newEvps, changed, err := outputs.UpdateProcessorInSlice(
		f.logger,
		f.store,
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
		f.dynCfg.Store(&newDC)
		f.logger.Printf("updated event processor %s", name)
	}
	return nil
}

// Write //
func (f *File) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	// load current config and file
	cfg := f.cfg.Load()
	dc := f.dynCfg.Load()
	fileHandle := f.file.Load()

	if cfg == nil || dc == nil || fileHandle == nil {
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

	numberOfReceivedMsgs.WithLabelValues(cfg.Name, (*fileHandle).Name()).Inc()

	rsp, err = outputs.AddSubscriptionTarget(rsp, meta, cfg.AddTarget, dc.targetTpl)
	if err != nil {
		f.logger.Printf("failed to add target to the response: %v", err)
	}

	bb, err := outputs.Marshal(rsp, meta, dc.mo, cfg.SplitEvents, dc.evps...)
	if err != nil {
		if cfg.Debug {
			f.logger.Printf("failed marshaling proto msg: %v", err)
		}
		numberOfFailWriteMsgs.WithLabelValues(cfg.Name, (*fileHandle).Name(), "marshal_error").Inc()
		return
	}
	if len(bb) == 0 {
		return
	}

	for _, b := range bb {
		if dc.msgTpl != nil {
			b, err = outputs.ExecTemplate(b, dc.msgTpl)
			if err != nil {
				if cfg.Debug {
					log.Printf("failed to execute template: %v", err)
				}
				numberOfFailWriteMsgs.WithLabelValues(cfg.Name, (*fileHandle).Name(), "template_error").Inc()
				continue
			}
		}

		n, err := (*fileHandle).Write(append(b, []byte(cfg.Separator)...))
		if err != nil {
			if cfg.Debug {
				f.logger.Printf("failed to write to file '%s': %v", (*fileHandle).Name(), err)
			}
			numberOfFailWriteMsgs.WithLabelValues(cfg.Name, (*fileHandle).Name(), "write_error").Inc()
			return
		}
		numberOfWrittenBytes.WithLabelValues(cfg.Name, (*fileHandle).Name()).Add(float64(n))
		numberOfWrittenMsgs.WithLabelValues(cfg.Name, (*fileHandle).Name()).Inc()
	}
}

func (f *File) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	// load current config and file
	cfg := f.cfg.Load()
	dc := f.dynCfg.Load()
	fileHandle := f.file.Load()

	if cfg == nil || dc == nil || fileHandle == nil {
		return
	}

	var evs = []*formatters.EventMsg{ev}
	for _, proc := range dc.evps {
		evs = proc.Apply(evs...)
	}

	toWrite := []byte{}
	if cfg.SplitEvents {
		for _, pev := range evs {
			var err error
			var b []byte
			if cfg.Multiline {
				b, err = json.MarshalIndent(pev, "", cfg.Indent)
			} else {
				b, err = json.Marshal(pev)
			}
			if err != nil {
				fmt.Printf("failed to WriteEvent: %v", err)
				numberOfFailWriteMsgs.WithLabelValues(cfg.Name, (*fileHandle).Name(), "marshal_error").Inc()
				return
			}
			toWrite = append(toWrite, b...)
			toWrite = append(toWrite, []byte(cfg.Separator)...)
		}
	} else {
		var err error
		var b []byte
		if cfg.Multiline {
			b, err = json.MarshalIndent(evs, "", cfg.Indent)
		} else {
			b, err = json.Marshal(evs)
		}
		if err != nil {
			fmt.Printf("failed to WriteEvent: %v", err)
			numberOfFailWriteMsgs.WithLabelValues(cfg.Name, (*fileHandle).Name(), "marshal_error").Inc()
			return
		}
		toWrite = append(toWrite, b...)
		toWrite = append(toWrite, []byte(cfg.Separator)...)
	}

	n, err := (*fileHandle).Write(toWrite)
	if err != nil {
		fmt.Printf("failed to WriteEvent: %v", err)
		numberOfFailWriteMsgs.WithLabelValues(cfg.Name, (*fileHandle).Name(), "write_error").Inc()
		return
	}
	numberOfWrittenBytes.WithLabelValues(cfg.Name, (*fileHandle).Name()).Add(float64(n))
	numberOfWrittenMsgs.WithLabelValues(cfg.Name, (*fileHandle).Name()).Inc()
}

// Close //
func (f *File) Close() error {
	fileHandle := f.file.Load()
	if fileHandle == nil {
		return nil
	}
	f.logger.Printf("closing file '%s' output", (*fileHandle).Name())
	return (*fileHandle).Close()
}

func (f *File) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(f.store)
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

func (f *File) setLogger(logger *log.Logger) {
	if logger != nil && f.logger != nil {
		f.logger.SetOutput(logger.Writer())
		f.logger.SetFlags(logger.Flags())
	}
}

func fileNeedsReopen(old, new *config) bool {
	if old == nil || new == nil {
		return true
	}
	// file needs to be reopened if file-related settings changed
	return old.FileName != new.FileName ||
		old.FileType != new.FileType ||
		rotationChanged(old.Rotation, new.Rotation)
}

func rotationChanged(old, new *rotationConfig) bool {
	if old == nil && new == nil {
		return false
	}
	if old == nil || new == nil {
		return true
	}
	// compare rotation config fields
	return old.MaxSize != new.MaxSize ||
		old.MaxAge != new.MaxAge ||
		old.MaxBackups != new.MaxBackups
}
