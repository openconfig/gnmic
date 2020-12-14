package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultFormat           = "json"
	defaultWriteConcurrency = 1000
	defaultSeparator        = "\n"
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
	mo      *collector.MarshalOptions
	sem     *semaphore.Weighted
}

// Config //
type Config struct {
	FileName         string `mapstructure:"filename,omitempty"`
	FileType         string `mapstructure:"file-type,omitempty"`
	Format           string `mapstructure:"format,omitempty"`
	Multiline        bool   `mapstructure:"multiline,omitempty"`
	Indent           string `mapstructure:"indent,omitempty"`
	Separator        string `mapstructure:"separator,omitempty"`
	ConcurrencyLimit int    `mapstructure:"concurrency-limit,omitempty"`
}

func (f *File) String() string {
	b, err := json.Marshal(f)
	if err != nil {
		return ""
	}
	return string(b)
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
	for _, opt := range opts {
		opt(f)
	}
	err := outputs.DecodeConfig(cfg, f.Cfg)
	if err != nil {
		f.logger.Printf("file output config decode failed: %v", err)
		return err
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

	if f.Cfg.ConcurrencyLimit < 1 {
		f.Cfg.ConcurrencyLimit = defaultWriteConcurrency
	}

	f.sem = semaphore.NewWeighted(int64(f.Cfg.ConcurrencyLimit))

	f.mo = &collector.MarshalOptions{Multiline: f.Cfg.Multiline, Indent: f.Cfg.Indent, Format: f.Cfg.Format}
	f.logger.Printf("initialized file output: %s", f.String())
	go func() {
		<-ctx.Done()
		f.Close()
	}()
	return nil
}

// Write //
func (f *File) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	err := f.sem.Acquire(ctx, 1)
	if errors.Is(err, context.Canceled) {
		return
	}

	if err != nil {
		f.logger.Printf("failed marshaling proto msg: %v", err)
		return
	}
	defer f.sem.Release(1)

	if rsp == nil {
		return
	}
	NumberOfReceivedMsgs.WithLabelValues(f.file.Name()).Inc()
	b, err := f.mo.Marshal(rsp, meta)
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
