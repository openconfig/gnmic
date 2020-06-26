package file

import (
	"log"
	"os"

	"github.com/karimra/gnmiClient/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
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
	Cfg      *Config
	file     *os.File
	logger   *log.Logger
	metrics  []prometheus.Collector
	stopChan chan struct{}
}

// Config //
type Config struct {
	FileName string
}

// Init //
func (f *File) Init(cfg map[string]interface{}, logger *log.Logger) error {
	c := new(Config)
	err := mapstructure.Decode(cfg, c)
	if err != nil {
		return err
	}
	f.Cfg = c
	file, err := os.OpenFile(f.Cfg.FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	f.file = file
	f.logger = log.New(os.Stderr, "file_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		f.logger.SetOutput(logger.Writer())
		f.logger.SetFlags(logger.Flags())
	}
	f.stopChan = make(chan struct{})
	f.logger.Printf("output initialized with config: %+v", f.Cfg)
	return nil
}

// Write //
func (f *File) Write(b []byte, meta outputs.Meta) {
	NumberOfReceivedMsgs.WithLabelValues(f.file.Name()).Inc()
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
	close(f.stopChan)
	return nil
}

// Metrics //
func (f *File) Metrics() []prometheus.Collector { return f.metrics }
