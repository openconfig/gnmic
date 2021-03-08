package file_loader

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/loaders"
	"gopkg.in/yaml.v2"
)

const (
	loggingPrefix = "[file_loader] "
	watchInterval = 5 * time.Second
)

func init() {
	loaders.Register("file", func() loaders.TargetLoader {
		return &FileLoader{
			cfg:    &cfg{},
			logger: log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

type FileLoader struct {
	cfg         *cfg
	lastTargets map[string]*collector.TargetConfig
	logger      *log.Logger
}

type cfg struct {
	File     string        `json:"file,omitempty" mapstructure:"file,omitempty"`
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`
}

func (f *FileLoader) Init(ctx context.Context, cfg map[string]interface{}) error {
	err := loaders.DecodeConfig(cfg, f.cfg)
	if err != nil {
		return err
	}
	if f.cfg.File == "" {
		return errors.New("missing file path")
	}
	if f.cfg.Interval <= 0 {
		f.cfg.Interval = watchInterval
	}
	return nil
}

func (f *FileLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	opChan := make(chan *loaders.TargetOperation)
	defer close(opChan)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				readTargets, err := f.readFile()
				if err != nil {
					f.logger.Printf("failed to read targets file: %v", err)
					time.Sleep(f.cfg.Interval)
					continue
				}
				select {
				case <-ctx.Done():
					return
				case opChan <- f.diff(readTargets):
					time.Sleep(f.cfg.Interval)
				}
			}
		}
	}()
	return opChan
}

func (f *FileLoader) readFile() (map[string]*collector.TargetConfig, error) {
	b, err := ioutil.ReadFile(f.cfg.File)
	if err != nil {
		return nil, err
	}
	readTargets := make(map[string]*collector.TargetConfig)
	switch filepath.Ext(f.cfg.File) {
	case ".json":
		err = json.Unmarshal(b, &readTargets)
		if err != nil {
			return nil, err
		}
	case ".yaml", ".yml":
		err = yaml.Unmarshal(b, &readTargets)
		if err != nil {
			return nil, err
		}
	}

	return readTargets, nil
}

func (f *FileLoader) diff(m map[string]*collector.TargetConfig) *loaders.TargetOperation {
	result := &loaders.TargetOperation{
		Add: make([]*collector.TargetConfig, 0),
		Del: make([]string, 0),
	}
	if len(f.lastTargets) == 0 {
		f.lastTargets = m
		for _, t := range m {
			result.Add = append(result.Add, t)
		}
		return result
	}
	if len(m) == 0 {
		for name := range f.lastTargets {
			result.Del = append(result.Del, name)
		}
		f.lastTargets = make(map[string]*collector.TargetConfig)
		return result
	}
	for n, t := range m {
		if _, ok := f.lastTargets[n]; !ok {
			f.lastTargets[n] = t
			result.Add = append(result.Add, t)
		}
	}
	for n := range f.lastTargets {
		if _, ok := m[n]; !ok {
			result.Del = append(result.Del, n)
		}
	}
	return result
}
