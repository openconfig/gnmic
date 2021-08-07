package file_loader

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/karimra/gnmic/loaders"
	"github.com/karimra/gnmic/types"
	"gopkg.in/yaml.v2"
)

const (
	loggingPrefix = "[file_loader] "
	watchInterval = 5 * time.Second
)

func init() {
	loaders.Register("file", func() loaders.TargetLoader {
		return &FileLoader{
			cfg:         &cfg{},
			lastTargets: make(map[string]*types.TargetConfig),
			logger:      log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

type FileLoader struct {
	cfg         *cfg
	lastTargets map[string]*types.TargetConfig
	logger      *log.Logger
}

type cfg struct {
	File     string        `json:"file,omitempty" mapstructure:"file,omitempty"`
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`
}

func (f *FileLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger) error {
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
	if logger != nil {
		f.logger.SetOutput(logger.Writer())
		f.logger.SetFlags(logger.Flags())
	}
	return nil
}

func (f *FileLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	opChan := make(chan *loaders.TargetOperation)
	go func() {
		defer close(opChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				readTargets, err := f.readFile()
				if _, ok := err.(*os.PathError); ok {
					time.Sleep(f.cfg.Interval)
					continue
				}
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

func (f *FileLoader) readFile() (map[string]*types.TargetConfig, error) {
	_, err := os.Stat(f.cfg.File)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(f.cfg.File)
	if err != nil {
		return nil, err
	}
	readTargets := make(map[string]*types.TargetConfig)
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
	for n, t := range readTargets {
		if t == nil {
			readTargets[n] = &types.TargetConfig{
				Name:    n,
				Address: n,
			}
			continue
		}
		if t.Name == "" {
			t.Name = n
		}
		if t.Address == "" {
			t.Address = n
		}
	}
	return readTargets, nil
}

func (f *FileLoader) diff(m map[string]*types.TargetConfig) *loaders.TargetOperation {
	result := loaders.Diff(f.lastTargets, m)
	for _, t := range result.Add {
		if _, ok := f.lastTargets[t.Name]; !ok {
			f.lastTargets[t.Name] = t
		}
	}
	for _, n := range result.Del {
		delete(f.lastTargets, n)
	}
	return result
}
