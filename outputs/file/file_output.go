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

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/proto"
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
			Cfg:    &Config{},
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// File //
type File struct {
	Cfg    *Config
	file   *os.File
	logger *log.Logger
	mo     *formatters.MarshalOptions
	sem    *semaphore.Weighted
	evps   []formatters.EventProcessor

	targetTpl *template.Template
	msgTpl    *template.Template
}

// Config //
type Config struct {
	FileName           string   `mapstructure:"filename,omitempty"`
	FileType           string   `mapstructure:"file-type,omitempty"`
	Format             string   `mapstructure:"format,omitempty"`
	Multiline          bool     `mapstructure:"multiline,omitempty"`
	Indent             string   `mapstructure:"indent,omitempty"`
	Separator          string   `mapstructure:"separator,omitempty"`
	OverrideTimestamps bool     `mapstructure:"override-timestamps,omitempty"`
	AddTarget          string   `mapstructure:"add-target,omitempty"`
	TargetTemplate     string   `mapstructure:"target-template,omitempty"`
	EventProcessors    []string `mapstructure:"event-processors,omitempty"`
	MsgTemplate        string   `mapstructure:"msg-template,omitempty"`
	ConcurrencyLimit   int      `mapstructure:"concurrency-limit,omitempty"`
	EnableMetrics      bool     `mapstructure:"enable-metrics,omitempty"`
	Debug              bool     `mapstructure:"debug,omitempty"`
}

func (f *File) String() string {
	b, err := json.Marshal(f)
	if err != nil {
		return ""
	}
	return string(b)
}

func (f *File) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) {
	for _, epName := range f.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType],
					formatters.WithLogger(logger),
					formatters.WithTargets(tcs),
					formatters.WithActions(acts),
				)
				if err != nil {
					f.logger.Printf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
					continue
				}
				f.evps = append(f.evps, ep)
				f.logger.Printf("added event processor '%s' of type=%s to file output", epName, epType)
				continue
			}
			f.logger.Printf("%q event processor has an unknown type=%q", epName, epType)
			continue
		}
		f.logger.Printf("%q event processor not found!", epName)
	}
}

func (f *File) SetLogger(logger *log.Logger) {
	if logger != nil && f.logger != nil {
		f.logger.SetOutput(logger.Writer())
		f.logger.SetFlags(logger.Flags())
	}
}

// Init //
func (f *File) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, f.Cfg)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(f)
	}
	if f.Cfg.Format == "proto" {
		return fmt.Errorf("proto format not supported in output type 'file'")
	}
	if f.Cfg.Separator == "" {
		f.Cfg.Separator = defaultSeparator
	}
	if f.Cfg.FileName == "" && f.Cfg.FileType == "" {
		f.Cfg.FileType = "stdout"
	}

	f.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))

	switch f.Cfg.FileType {
	case "stdout":
		f.file = os.Stdout
	case "stderr":
		f.file = os.Stderr
	default:
	CRFILE:
		f.file, err = os.OpenFile(f.Cfg.FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			f.logger.Printf("failed to create file: %v", err)
			time.Sleep(10 * time.Second)
			goto CRFILE
		}
	}

	if f.Cfg.Format == "" {
		f.Cfg.Format = defaultFormat
	}
	if f.Cfg.FileType == "stdout" || f.Cfg.FileType == "stderr" {
		f.Cfg.Indent = "  "
		f.Cfg.Multiline = true
	}
	if f.Cfg.Multiline && f.Cfg.Indent == "" {
		f.Cfg.Indent = "  "
	}
	if f.Cfg.ConcurrencyLimit < 1 {
		f.Cfg.ConcurrencyLimit = defaultWriteConcurrency
	}

	f.sem = semaphore.NewWeighted(int64(f.Cfg.ConcurrencyLimit))

	f.mo = &formatters.MarshalOptions{
		Multiline:  f.Cfg.Multiline,
		Indent:     f.Cfg.Indent,
		Format:     f.Cfg.Format,
		OverrideTS: f.Cfg.OverrideTimestamps,
	}
	if f.Cfg.TargetTemplate == "" {
		f.targetTpl = outputs.DefaultTargetTemplate
	} else if f.Cfg.AddTarget != "" {
		f.targetTpl, err = utils.CreateTemplate("target-template", f.Cfg.TargetTemplate)
		if err != nil {
			return err
		}
		f.targetTpl = f.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	if f.Cfg.MsgTemplate != "" {
		f.msgTpl, err = utils.CreateTemplate(fmt.Sprintf("%s-msg-template", name), f.Cfg.MsgTemplate)
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

	NumberOfReceivedMsgs.WithLabelValues(f.file.Name()).Inc()
	err = outputs.AddSubscriptionTarget(rsp, meta, f.Cfg.AddTarget, f.targetTpl)
	if err != nil {
		f.logger.Printf("failed to add target to the response: %v", err)
	}

	b, err := f.mo.Marshal(rsp, meta, f.evps...)
	if err != nil {
		if f.Cfg.Debug {
			f.logger.Printf("failed marshaling proto msg: %v", err)
		}
		NumberOfFailWriteMsgs.WithLabelValues(f.file.Name(), "marshal_error").Inc()
		return
	}

	if f.msgTpl != nil && len(b) > 0 {
		b, err = outputs.ExecTemplate(b, f.msgTpl)
		if err != nil {
			if f.Cfg.Debug {
				log.Printf("failed to execute template: %v", err)
			}
			NumberOfFailWriteMsgs.WithLabelValues(f.file.Name(), "template_error").Inc()
			return
		}
	}

	n, err := f.file.Write(append(b, []byte(f.Cfg.Separator)...))
	if err != nil {
		if f.Cfg.Debug {
			f.logger.Printf("failed to write to file '%s': %v", f.file.Name(), err)
		}
		NumberOfFailWriteMsgs.WithLabelValues(f.file.Name(), "write_error").Inc()
		return
	}
	NumberOfWrittenBytes.WithLabelValues(f.file.Name()).Add(float64(n))
	NumberOfWrittenMsgs.WithLabelValues(f.file.Name()).Inc()
}

func (f *File) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

// Close //
func (f *File) Close() error {
	f.logger.Printf("closing file '%s' output", f.file.Name())
	return f.file.Close()
}

// Metrics //
func (f *File) RegisterMetrics(reg *prometheus.Registry) {
	if !f.Cfg.EnableMetrics {
		return
	}
	if err := registerMetrics(reg); err != nil {
		f.logger.Printf("failed to register metric: %v", err)
	}
}

func (f *File) SetName(name string)                             {}
func (f *File) SetClusterName(name string)                      {}
func (f *File) SetTargetsConfig(map[string]*types.TargetConfig) {}
