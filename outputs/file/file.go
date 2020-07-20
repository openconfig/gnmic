package file

import (
	"encoding/json"
	"log"
	"os"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
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
}

// Config //
type Config struct {
	FileName string `mapstructure:"filename,omitempty"`
	FileType string `mapstructure:"file-type,omitempty"`
	Format   string `mapstructure:"format,omitempty"`
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
	c := new(Config)
	err := mapstructure.Decode(cfg, c)
	if err != nil {
		return err
	}
	f.Cfg = c

	var file *os.File
	switch f.Cfg.FileType {
	case "stdout":
		file = os.Stdout
	case "stderr":
		file = os.Stderr
	default:
		file, err = os.OpenFile(f.Cfg.FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
	}
	f.file = file
	f.logger = log.New(os.Stderr, "file_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		f.logger.SetOutput(logger.Writer())
		f.logger.SetFlags(logger.Flags())
	}
	if f.Cfg.Format == "" {
		f.Cfg.Format = "event"
	}
	f.logger.Printf("initialized file output: %s", f.String())
	return nil
}

// Write //
func (f *File) Write(rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	NumberOfReceivedMsgs.WithLabelValues(f.file.Name()).Inc()
	b := make([]byte, 0)
	var err error
	switch f.Cfg.Format {
	case "proto":
		b, err = proto.Marshal(rsp)
	case "json":
		if f.Cfg.FileType == "stdout" || f.Cfg.FileType == "stderr" {
			b, err = protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(rsp)
		} else {
			b, err = protojson.Marshal(rsp)
		}
	case "textproto":
		b, err = prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(rsp)
	case "event":
		switch sub := rsp.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeResponse:
			var subscriptionName string
			var ok bool
			if subscriptionName, ok = meta["subscription-name"]; !ok {
				subscriptionName = "default"
			}
			switch sub.Response.(type) {
			case *gnmi.SubscribeResponse_Update:
				events, err := collector.ResponseToEventMsgs(subscriptionName, sub, meta)
				if err != nil {
					f.logger.Printf("failed converting response to events: %v", err)
					return
				}
				if f.Cfg.FileType == "stdout" || f.Cfg.FileType == "stderr" {
					b, err = json.MarshalIndent(events, "", "  ")
				} else {
					b, err = json.Marshal(events)
				}
				if err != nil {
					f.logger.Printf("failed marshaling events: %v", err)
					return
				}
			case *gnmi.SubscribeResponse_SyncResponse:
				f.logger.Printf("received subscribe syncResponse with %v", meta)
			case *gnmi.SubscribeResponse_Error:
				gnmiErr := sub.GetError()
				f.logger.Printf("received subscribe response error with %v, code=%d, message=%v, data=%v ",
					meta, gnmiErr.Code, gnmiErr.Message, gnmiErr.Data)
			}
		}
	}
	if err != nil {
		f.logger.Printf("failed marshaling event: %v", err)
		return
	}
	n, err := f.file.Write(append(b, []byte("\n")...))
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
