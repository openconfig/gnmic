package file

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
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
	mo      *collector.MarshalOptions
}

// Config //
type Config struct {
	FileName  string `mapstructure:"filename,omitempty"`
	FileType  string `mapstructure:"file-type,omitempty"`
	Format    string `mapstructure:"format,omitempty"`
	Multiline bool   `mapstructure:"multiline,omitempty"`
	Indent    string `mapstructure:"indent,omitempty"`
	Separator string `mapstructure:"separator,omitempty"`
}

func (f *File) String() string {
	b, err := json.Marshal(f)
	if err != nil {
		return ""
	}
	return string(b)
}

// Init //
func (f *File) Init(cfg map[string]interface{}, logger *log.Logger) error {
	err := mapstructure.Decode(cfg, f.Cfg)
	if err != nil {
		return err
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
	switch f.Cfg.FileType {
	case "stdout":
		f.file = os.Stdout
	case "stderr":
		f.file = os.Stderr
	default:
		f.file, err = os.OpenFile(f.Cfg.FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
	}
	f.logger = log.New(os.Stderr, "file_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		f.logger.SetOutput(logger.Writer())
		f.logger.SetFlags(logger.Flags())
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
	f.mo = &collector.MarshalOptions{Multiline: f.Cfg.Multiline, Indent: f.Cfg.Indent, Format: f.Cfg.Format}
	f.logger.Printf("initialized file output: %s", f.String())
	return nil
}

// Write //
func (f *File) Write(rsp proto.Message, meta outputs.Meta) {
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
