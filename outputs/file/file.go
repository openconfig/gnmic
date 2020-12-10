package file

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultFormat    = "json"
	defaultSeparator = "\n"
)

func init() {
	outputs.Register("file", func() outputs.Output {
		return &File{
			Cfg: &Config{},
			metrics: []prometheus.Collector{
				NumberOfWrittenBytes,
				NumberOfReceivedMsgs,
				NumberOfWrittenMsgs,
			},
		}
	})
}

// File //
type File struct {
	Cfg     *Config
	file    *os.File
	logger  *log.Logger
	metrics []prometheus.Collector
	mo      *formatters.MarshalOptions
	evps    []formatters.EventProcessor
}

// Config //
type Config struct {
	FileName        string   `mapstructure:"filename,omitempty"`
	FileType        string   `mapstructure:"file-type,omitempty"`
	Format          string   `mapstructure:"format,omitempty"`
	Multiline       bool     `mapstructure:"multiline,omitempty"`
	Indent          string   `mapstructure:"indent,omitempty"`
	Separator       string   `mapstructure:"separator,omitempty"`
	EventProcessors []string `mapstructure:"event-processors,omitempty"`
}

func (f *File) String() string {
	b, err := json.Marshal(f)
	if err != nil {
		return ""
	}
	return string(b)
}

func (f *File) SetEventProcessors(ps map[string]map[string]interface{}, log *log.Logger) {
	for _, epName := range f.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType], log)
				if err != nil {
					f.logger.Printf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
					continue
				}
				f.evps = append(f.evps, ep)
				f.logger.Printf("added event processor '%s' of type=%s to file output", epName, epType)
			}
		}
	}
}

func (f *File) SetLogger(logger *log.Logger) {
	if logger != nil {
		f.logger = log.New(logger.Writer(), "file_output ", logger.Flags())
		return
	}
	f.logger = log.New(os.Stderr, "file_output ", log.LstdFlags|log.Lmicroseconds)
}

// Init //
func (f *File) Init(ctx context.Context, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, f.Cfg)
	if err != nil {
		f.logger.Printf("file output config decode failed: %v", err)
		return err
	}
	for _, opt := range opts {
		opt(f)
	}
	if f.Cfg.Format == "proto" {
		f.logger.Printf("proto format not supported in output type 'file'")
		return fmt.Errorf("proto format not supported in output type 'file'")
	}
	if f.Cfg.Separator == "" {
		f.Cfg.Separator = defaultSeparator
	}
	if f.Cfg.FileName == "" && f.Cfg.FileType == "" {
		f.Cfg.FileType = "stdout"
	}
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
	f.mo = &formatters.MarshalOptions{Multiline: f.Cfg.Multiline, Indent: f.Cfg.Indent, Format: f.Cfg.Format}
	f.logger.Printf("initialized file output: %s", f.String())
	go func() {
		<-ctx.Done()
		f.Close()
	}()
	return nil
}

// Write //
func (f *File) Write(_ context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	NumberOfReceivedMsgs.WithLabelValues(f.file.Name()).Inc()
	b, err := f.mo.Marshal(rsp, meta, f.evps...)
	if err != nil {
		f.logger.Printf("failed marshaling proto msg: %v", err)
		return
	}
	n, err := f.file.Write(append(b, []byte(f.Cfg.Separator)...))
	if err != nil {
		f.logger.Printf("failed to write to file '%s': %v", f.file.Name(), err)
		return
	}
	NumberOfWrittenBytes.WithLabelValues(f.file.Name()).Add(float64(n))
	NumberOfWrittenMsgs.WithLabelValues(f.file.Name()).Inc()
}

// Close //
func (f *File) Close() error {
	f.logger.Printf("closing file '%s' output", f.file.Name())
	return f.file.Close()
}

// Metrics //
func (f *File) Metrics() []prometheus.Collector { return f.metrics }
