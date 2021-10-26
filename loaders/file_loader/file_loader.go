package file_loader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/karimra/gnmic/loaders"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
)

const (
	loggingPrefix = "[file_loader] "
	watchInterval = 30 * time.Second
	loaderType    = "file"
)

func init() {
	loaders.Register(loaderType, func() loaders.TargetLoader {
		return &fileLoader{
			cfg:         &cfg{},
			lastTargets: make(map[string]*types.TargetConfig),
			logger:      log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// fileLoader implements the loaders.Loader interface.
// it reads a configured file (local, ftp, sftp, http) periodically, expects the file to contain
// a dictionnary of types.TargetConfig.
// It then adds new targets to gNMIc's targets and deletes the removes ones.
type fileLoader struct {
	cfg         *cfg
	lastTargets map[string]*types.TargetConfig
	logger      *log.Logger
}

type cfg struct {
	// path the the file, if remote,
	// must include the proper protocol prefix ftp://, sftp://, http://
	Path string `json:"path,omitempty" mapstructure:"path,omitempty"`
	// the interval at which the file will be re read to load new targets
	// or delete removed ones.
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`
	// if true, registers fileLoader prometheus metrics with the provided
	// prometheus registry
	EnableMetrics bool `json:"enable-metrics,omitempty" mapstructure:"enable-metrics,omitempty"`
}

func (f *fileLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger, opts ...loaders.Option) error {
	err := loaders.DecodeConfig(cfg, f.cfg)
	if err != nil {
		return err
	}
	for _, o := range opts {
		o(f)
	}
	if f.cfg.Path == "" {
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

func (f *fileLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	opChan := make(chan *loaders.TargetOperation)
	go func() {
		defer close(opChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				readTargets, err := f.getTargets(ctx)
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

func (f *fileLoader) RegisterMetrics(reg *prometheus.Registry) {
	if !f.cfg.EnableMetrics && reg != nil {
		return
	}
	if err := registerMetrics(reg); err != nil {
		f.logger.Printf("failed to register metrics: %v", err)
	}
}

func (f *fileLoader) getTargets(ctx context.Context) (map[string]*types.TargetConfig, error) {
	fileLoaderFileReadTotal.WithLabelValues(loaderType).Add(1)
	start := time.Now()
	// read file bytes based on the path prefix
	ctx, cancel := context.WithTimeout(ctx, f.cfg.Interval/2)
	defer cancel()
	b, err := utils.ReadFile(ctx, f.cfg.Path)
	fileLoaderFileReadDuration.WithLabelValues(loaderType).Set(float64(time.Since(start).Nanoseconds()))
	if err != nil {
		fileLoaderFailedFileRead.WithLabelValues(loaderType, fmt.Sprintf("%v", err)).Add(1)
		return nil, err
	}
	result := make(map[string]*types.TargetConfig)
	// unmarshal the bytes into a map of targetConfigs
	err = yaml.Unmarshal(b, result)
	if err != nil {
		fileLoaderFailedFileRead.WithLabelValues(loaderType, fmt.Sprintf("%v", err)).Add(1)
		return nil, err
	}
	// properly initialize address and name if not set
	for n, t := range result {
		if t == nil && n != "" {
			result[n] = &types.TargetConfig{
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
	return result, nil
}

// diff compares the given map[string]*types.TargetConfig with the
// stored f.lastTargets and returns
func (f *fileLoader) diff(m map[string]*types.TargetConfig) *loaders.TargetOperation {
	result := loaders.Diff(f.lastTargets, m)
	for _, t := range result.Add {
		if _, ok := f.lastTargets[t.Name]; !ok {
			f.lastTargets[t.Name] = t
		}
	}
	for _, n := range result.Del {
		delete(f.lastTargets, n)
	}
	fileLoaderLoadedTargets.WithLabelValues(loaderType).Set(float64(len(result.Add)))
	fileLoaderDeletedTargets.WithLabelValues(loaderType).Set(float64(len(result.Del)))
	return result
}
