package file_loader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"text/template"
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
// expects the file to contain a dictionary of types.TargetConfig.
// It then adds new targets to gNMIc's targets and deletes the removed ones.
type fileLoader struct {
	cfg            *cfg
	m              *sync.RWMutex
	lastTargets    map[string]*types.TargetConfig
	targetConfigFn func(*types.TargetConfig) error
	logger         *log.Logger
	//
	tpl           *template.Template
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
	// a Go text template that can be used to transform the targets format read from the file to match
	// gNMIc's expected format.
	Template string `json:"template,omitempty" mapstructure:"template,omitempty"`
	// time to wait before the first file read
	StartDelay time.Duration `json:"start-delay,omitempty" mapstructure:"start-delay,omitempty"`
	// if true, registers fileLoader prometheus metrics with the provided
	// prometheus registry
	EnableMetrics bool `json:"enable-metrics,omitempty" mapstructure:"enable-metrics,omitempty"`
	// enable Debug
	Debug bool `json:"debug,omitempty" mapstructure:"debug,omitempty"`
	// variables definitions to be passed to the actions
	Vars map[string]interface{} `json:"vars,omitempty" mapstructure:"vars,omitempty"`
	// variable file, values in this file will be overwritten by
	// the ones defined in Vars
	VarsFile string `json:"vars-file,omitempty" mapstructure:"vars-file,omitempty"`
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
	if f.cfg.Template != "" {
		f.tpl, err = utils.CreateTemplate("file-loader-template", f.cfg.Template)
		if err != nil {
			return err
		}
	}
	err = f.readVars(ctx)
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
	f.logger.Printf("initialized loader type %q: %s", loaderType, f)
	return nil
}

func (f *fileLoader) String() string {
	b, err := json.Marshal(f.cfg)
	if err != nil {
		return fmt.Sprintf("%+v", f.cfg)
	}
	return string(b)
}

func (f *fileLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	opChan := make(chan *loaders.TargetOperation)
	ticker := time.NewTicker(f.cfg.Interval)
	go func() {
		defer close(opChan)
		defer ticker.Stop()
		time.Sleep(f.cfg.StartDelay)
		f.update(ctx, opChan)
		for {
			select {
			case <-ctx.Done():
				f.logger.Printf("%q context done: %v", loaderType, ctx.Err())
				return
			case <-ticker.C:
				f.update(ctx, opChan)
			}
		}
	}()
	return opChan
}

func (f *fileLoader) RunOnce(ctx context.Context) (map[string]*types.TargetConfig, error) {
	readTargets, err := f.getTargets(ctx)
	if err != nil {
		return nil, err
	}
	if f.cfg.Debug {
		f.logger.Printf("file loader discovered %d target(s)", len(readTargets))
	}
	return readTargets, nil
}

func (f *fileLoader) update(ctx context.Context, opChan chan *loaders.TargetOperation) {
	readTargets, err := f.RunOnce(ctx)
	if _, ok := err.(*os.PathError); ok {
		f.logger.Printf("path err: %v", err)
		return
	}
	if err != nil {
		f.logger.Printf("failed to read targets file: %v", err)
		return
	}
	select {
	// check if the context is done before
	// updating the targets to the channel
	case <-ctx.Done():
		f.logger.Printf("context done: %v", ctx.Err())
		return
	default:
		f.updateTargets(ctx, readTargets, opChan)
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
	if f.tpl != nil {
		var input interface{}
		err = json.Unmarshal(b, input)
		if err != nil {
			fileLoaderFailedFileRead.WithLabelValues(loaderType, fmt.Sprintf("%v", err)).Add(1)
			return nil, err
		}
		buf := new(bytes.Buffer)
		err = f.tpl.Execute(buf, input)
		if err != nil {
			fileLoaderFailedFileRead.WithLabelValues(loaderType, fmt.Sprintf("%v", err)).Add(1)
			return nil, err
		}
		b = buf.Bytes()
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
	f.logger.Printf("result: %s", result)
	return result, nil
}

func (f *fileLoader) updateTargets(ctx context.Context, tcs map[string]*types.TargetConfig, opChan chan *loaders.TargetOperation) {
	var err error
	for _, tc := range tcs {
		err = f.targetConfigFn(tc)
		if err != nil {
			f.logger.Printf("failed running target config fn on target %q", tc.Name)
		}
	}
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

func (f *fileLoader) readVars(ctx context.Context) error {
	if f.cfg.VarsFile == "" {
		f.vars = f.cfg.Vars
		return nil
	}
	b, err := utils.ReadFile(ctx, f.cfg.VarsFile)
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
	if f.numActions == 0 {
		return targetOp, nil
	}
	opChan := make(chan *loaders.TargetOperation)
	// some actions are defined,
	doneCh := make(chan struct{})
	result := &loaders.TargetOperation{
		Add: make([]*types.TargetConfig, 0, len(targetOp.Add)),
		Del: make([]string, 0, len(targetOp.Del)),
	}
	ctx, cancel := context.WithTimeout(ctx, f.cfg.Interval)
	defer cancel()
	// start gathering goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(doneCh)
				return
			case op, ok := <-opChan:
				if !ok {
					close(doneCh)
					return
				}
				result.Add = append(result.Add, op.Add...)
				result.Del = append(result.Del, op.Del...)
			}
		}
	}()
	// create waitGroup and add the number of target operations to it
	wg := new(sync.WaitGroup)
	wg.Add(len(targetOp.Add) + len(targetOp.Del))
	// run OnAdd actions
	for _, tAdd := range targetOp.Add {
		go func(tc *types.TargetConfig) {
			defer wg.Done()
			err := f.runOnAddActions(ctx, tc.Name, tcs)
			if err != nil {
				f.logger.Printf("failed running OnAdd actions: %v", err)
				return
			}
			opChan <- &loaders.TargetOperation{Add: []*types.TargetConfig{tc}}
		}(tAdd)
	}
	// run OnDelete actions
	for _, tDel := range targetOp.Del {
		go func(name string) {
			defer wg.Done()
			err := f.runOnDeleteActions(ctx, name, tcs)
			if err != nil {
				f.logger.Printf("failed running OnDelete actions: %v", err)
				return
			}
			opChan <- &loaders.TargetOperation{Del: []string{name}}
		}(tDel)
	}
	wg.Wait()
	close(opChan)
	<-doneCh //wait for gathering goroutine to finish
	return result, nil
}

func (d *fileLoader) runOnAddActions(ctx context.Context, tName string, tcs map[string]*types.TargetConfig) error {
	aCtx := &actions.Context{
		Input:   tName,
		Env:     make(map[string]interface{}),
		Vars:    d.vars,
		Targets: tcs,
	}
	for _, act := range d.addActions {
		d.logger.Printf("running action %q for target %q", act.NName(), tName)
		res, err := act.Run(ctx, aCtx)
		if err != nil {
			// delete target from known targets map
			d.m.Lock()
			delete(d.lastTargets, tName)
			d.m.Unlock()
			return fmt.Errorf("action %q for target %q failed: %v", act.NName(), tName, err)
		}

		aCtx.Env[act.NName()] = utils.Convert(res)
		if d.cfg.Debug {
			d.logger.Printf("action %q, target %q result: %+v", act.NName(), tName, res)
			b, _ := json.MarshalIndent(aCtx, "", "  ")
			d.logger.Printf("action %q context:\n%s", act.NName(), string(b))
		}
	}
	return nil
}

func (d *fileLoader) runOnDeleteActions(ctx context.Context, tName string, tcs map[string]*types.TargetConfig) error {
	env := make(map[string]interface{})
	for _, act := range d.delActions {
		res, err := act.Run(ctx, &actions.Context{Input: tName, Env: env, Vars: d.vars})
		if err != nil {
			return fmt.Errorf("action %q for target %q failed: %v", act.NName(), tName, err)
		}
		env[act.NName()] = res
	}
	return nil
}
