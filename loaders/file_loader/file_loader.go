package file_loader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/karimra/gnmic/actions"
	"github.com/karimra/gnmic/loaders"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
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
			m:           new(sync.RWMutex),
			lastTargets: make(map[string]*types.TargetConfig),
			logger:      log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// fileLoader implements the loaders.Loader interface.
// it reads a configured file (local, ftp, sftp, http) periodically,
// expects the file to contain a dictionnary of types.TargetConfig.
// It then adds new targets to gNMIc's targets and deletes the removed ones.
type fileLoader struct {
	cfg            *cfg
	m              *sync.RWMutex
	lastTargets    map[string]*types.TargetConfig
	targetConfigFn func(*types.TargetConfig) error
	logger         *log.Logger
	//
	vars          map[string]interface{}
	actionsConfig map[string]map[string]interface{}
	addActions    []actions.Action
	delActions    []actions.Action
	numActions    int
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
	// variables definitions to be passed to the actions
	Vars map[string]interface{}
	// variable file, values in this file will be overwritten by
	// the ones defined in Vars
	VarsFile string `mapstructure:"vars-file,omitempty"`
	// run onAdd and onDelete actions asynchronously
	Async bool `json:"async,omitempty" mapstructure:"async,omitempty"`
	// list of Actions to run on new target discovery
	OnAdd []string `json:"on-add,omitempty" mapstructure:"on-add,omitempty"`
	// list of Actions to run on target removal
	OnDelete []string `json:"on-delete,omitempty" mapstructure:"on-delete,omitempty"`
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
	err = f.readVars()
	if err != nil {
		return err
	}
	for _, actName := range f.cfg.OnAdd {
		if cfg, ok := f.actionsConfig[actName]; ok {
			a, err := f.initializeAction(cfg)
			if err != nil {
				return err
			}
			f.addActions = append(f.addActions, a)
			continue
		}
		return fmt.Errorf("unknown action name %q", actName)

	}
	for _, actName := range f.cfg.OnDelete {
		if cfg, ok := f.actionsConfig[actName]; ok {
			a, err := f.initializeAction(cfg)
			if err != nil {
				return err
			}
			f.delActions = append(f.delActions, a)
			continue
		}
		return fmt.Errorf("unknown action name %q", actName)
	}
	f.numActions = len(f.addActions) + len(f.delActions)
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
				default:
					f.updateTargets(ctx, readTargets, opChan)
					time.Sleep(f.cfg.Interval)
				}
			}
		}
	}()
	return opChan
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
func (f *fileLoader) updateTargets(ctx context.Context, tcs map[string]*types.TargetConfig, opChan chan *loaders.TargetOperation) {
	targetOp, err := f.runActions(ctx, tcs, loaders.Diff(f.lastTargets, tcs))
	if err != nil {
		f.logger.Printf("failed to run actions: %v", err)
		return
	}
	numAdds := len(targetOp.Add)
	numDels := len(targetOp.Del)
	defer func() {
		fileLoaderLoadedTargets.WithLabelValues(loaderType).Set(float64(numAdds))
		fileLoaderDeletedTargets.WithLabelValues(loaderType).Set(float64(numDels))
	}()
	if numAdds+numDels == 0 {
		return
	}
	f.m.Lock()
	for _, add := range targetOp.Add {
		f.lastTargets[add.Name] = add
	}
	for _, del := range targetOp.Del {
		delete(f.lastTargets, del)
	}
	f.m.Unlock()
	opChan <- targetOp
}

func (f *fileLoader) readVars() error {
	if f.cfg.VarsFile == "" {
		f.vars = f.cfg.Vars
		return nil
	}
	b, err := utils.ReadFile(context.TODO(), f.cfg.VarsFile)
	if err != nil {
		return err
	}
	v := make(map[string]interface{})
	err = yaml.Unmarshal(b, &v)
	if err != nil {
		return err
	}
	f.vars = utils.MergeMaps(v, f.cfg.Vars)
	return nil
}

func (f *fileLoader) initializeAction(cfg map[string]interface{}) (actions.Action, error) {
	if len(cfg) == 0 {
		return nil, errors.New("missing action definition")
	}
	if actType, ok := cfg["type"]; ok {
		switch actType := actType.(type) {
		case string:
			if in, ok := actions.Actions[actType]; ok {
				act := in()
				err := act.Init(cfg, actions.WithLogger(f.logger), actions.WithTargets(nil))
				if err != nil {
					return nil, err
				}

				return act, nil
			}
			return nil, fmt.Errorf("unknown action type %q", actType)
		default:
			return nil, fmt.Errorf("unexpected action field type %T", actType)
		}
	}
	return nil, errors.New("missing type field under action")
}

func (f *fileLoader) runActions(ctx context.Context, tcs map[string]*types.TargetConfig, targetOp *loaders.TargetOperation) (*loaders.TargetOperation, error) {
	return nil, nil
}
