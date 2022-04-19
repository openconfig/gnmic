package consul_loader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"

	"github.com/karimra/gnmic/actions"
	"github.com/karimra/gnmic/loaders"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

const (
	loggingPrefix  = "[consul_loader] "
	loaderType     = "consul"
	defaultAddress = "localhost:8500"
	defaultPrefix  = "gnmic/config/targets"
	//
	defaultWatchTimeout  = 1 * time.Minute
	defaultActionTimeout = 30 * time.Second
)

func init() {
	loaders.Register(loaderType, func() loaders.TargetLoader {
		return &consulLoader{
			cfg:         &cfg{},
			m:           new(sync.Mutex),
			lastTargets: make(map[string]*types.TargetConfig),
			logger:      log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type consulLoader struct {
	cfg *cfg
	// decoder        *consulstructure.Decoder
	client         *api.Client
	m              *sync.Mutex
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
	// Consul server address
	Address string `mapstructure:"address,omitempty" json:"address,omitempty"`
	// Consul datacenter name, defaults to dc1
	Datacenter string `mapstructure:"datacenter,omitempty" json:"datacenter,omitempty"`
	// Consul username
	Username string `mapstructure:"username,omitempty" json:"username,omitempty"`
	// Consul Password
	Password string `mapstructure:"password,omitempty" json:"password,omitempty"`
	// Consul token
	Token string `mapstructure:"token,omitempty" json:"token,omitempty"`
	// enable debug
	Debug bool `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	// KV based target config loading
	KeyPrefix string `mapstructure:"key-prefix,omitempty" json:"key-prefix,omitempty"`
	// Service based target config loading
	Services []*serviceDef `mapstructure:"services,omitempty" json:"services,omitempty"`
	// if true, registers consulLoader prometheus metrics with the provided
	// prometheus registry
	EnableMetrics bool `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	// variables definitions to be passed to the actions
	Vars map[string]interface{}
	// variable file, values in this file will be overwritten by
	// the ones defined in Vars
	VarsFile string `mapstructure:"vars-file,omitempty" json:"vars-file,omitempty"`
	// list of Actions to run on new target discovery
	OnAdd []string `mapstructure:"on-add,omitempty" json:"on-add,omitempty"`
	// list of Actions to run on target removal
	OnDelete []string `mapstructure:"on-delete,omitempty" json:"on-delete,omitempty"`
	// timeout for the actions, this applies for all actions as a whole (on-add + on-delete),
	// not to each action individually.
	ActionsTimeout time.Duration `mapstructure:"actions-timeout,omitempty" json:"actions-timeout,omitempty"`
}

type serviceDef struct {
	Name   string                 `mapstructure:"name,omitempty" json:"name,omitempty"`
	Tags   []string               `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Config map[string]interface{} `mapstructure:"config,omitempty" json:"config,omitempty"`
}

func (c *consulLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger, opts ...loaders.Option) error {
	err := loaders.DecodeConfig(cfg, c.cfg)
	if err != nil {
		return err
	}
	err = c.setDefaults()
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(c)
	}
	if logger != nil {
		c.logger.SetOutput(logger.Writer())
		c.logger.SetFlags(logger.Flags())
	}
	err = c.readVars(ctx)
	if err != nil {
		return err
	}
	for _, actName := range c.cfg.OnAdd {
		if cfg, ok := c.actionsConfig[actName]; ok {
			a, err := c.initializeAction(cfg)
			if err != nil {
				return err
			}
			c.addActions = append(c.addActions, a)
			continue
		}
		return fmt.Errorf("unknown action name %q", actName)

	}
	for _, actName := range c.cfg.OnDelete {
		if cfg, ok := c.actionsConfig[actName]; ok {
			a, err := c.initializeAction(cfg)
			if err != nil {
				return err
			}
			c.delActions = append(c.delActions, a)
			continue
		}
		return fmt.Errorf("unknown action name %q", actName)
	}
	c.numActions = len(c.addActions) + len(c.delActions)
	c.logger.Printf("intialized consul loader: %+v", c.cfg)
	return nil
}

func (c *consulLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	opChan := make(chan *loaders.TargetOperation)
	var err error
CLIENT:
	err = c.initClient()
	if err != nil {
		c.logger.Printf("Failed to create a Consul client:%v", err)
		consulLoaderWatchError.WithLabelValues(loaderType, fmt.Sprintf("%v", err)).Add(1)
		time.Sleep(2 * time.Second)
		goto CLIENT
	}
	sChan := make(chan []*api.ServiceEntry)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ses, ok := <-sChan:
				if !ok {
					return
				}
				tcs := make(map[string]*types.TargetConfig)
				for _, se := range ses {
					tc, err := c.serviceEntryToTargetConfig(se)
					if err != nil {
						c.logger.Printf("Failed to convert service entry %+v to a target config: %v", se, err)
					}
					tcs[tc.Name] = tc
				}
				c.updateTargets(ctx, tcs, opChan)
			}
		}
	}()
	for _, s := range c.cfg.Services {
		go func(s *serviceDef) {
			err := c.startServicesWatch(ctx, s.Name, s.Tags, sChan, time.Minute)
			if err != nil {
				c.logger.Printf("service %q watch stopped: %v", s.Name, err)
			}
		}(s)
	}
	return opChan
}

func (c *consulLoader) RunOnce(ctx context.Context) (map[string]*types.TargetConfig, error) {
	err := c.initClient()
	if err != nil {
		return nil, err
	}
	result := make(map[string]*types.TargetConfig)
	rsChan := make(chan *api.ServiceEntry)
	m := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(len(c.cfg.Services))

	for _, s := range c.cfg.Services {
		go func(s *serviceDef) {
			ses, _, err := c.client.Health().ServiceMultipleTags(s.Name, s.Tags, true, &api.QueryOptions{})
			if err != nil {
				c.logger.Printf("failed to get service %q instances: %v", s.Name, err)
				return
			}
			for _, se := range ses {
				rsChan <- se
			}
		}(s)
	}

	go func() {
		m.Lock()
		defer m.Unlock()
		for {
			select {
			case se, ok := <-rsChan:
				if !ok {
					return
				}
				tc, err := c.serviceEntryToTargetConfig(se)
				if err != nil {
					c.logger.Printf("failed to convert service %+v to target config: %v", se, err)
				}
				result[tc.Name] = tc
			case <-ctx.Done():
				return
			}
		}
	}()
	wg.Wait()
	close(rsChan)
	m.Lock()
	defer m.Unlock()
	return result, nil
}

//

func (c *consulLoader) initClient() error {
	var err error
	if c.client != nil {
		_, err = c.client.Agent().Self()
		if err == nil {
			return nil
		}
	}
	// create a new client
	clientConfig := &api.Config{
		Address:    c.cfg.Address,
		Scheme:     "http",
		Datacenter: c.cfg.Datacenter,
		Token:      c.cfg.Token,
	}
	if c.cfg.Username != "" && c.cfg.Password != "" {
		clientConfig.HttpAuth = &api.HttpBasicAuth{
			Username: c.cfg.Username,
			Password: c.cfg.Password,
		}
	}
	c.client, err = api.NewClient(clientConfig)
	return err
}

func (c *consulLoader) setDefaults() error {
	if c.cfg.Address == "" {
		c.cfg.Address = defaultAddress
	}
	if c.cfg.Datacenter == "" {
		c.cfg.Datacenter = "dc1"
	}
	if c.cfg.KeyPrefix == "" && len(c.cfg.Services) == 0 {
		c.cfg.KeyPrefix = defaultPrefix
	}
	if c.cfg.ActionsTimeout <= 0 {
		c.cfg.ActionsTimeout = defaultActionTimeout
	}
	return nil
}

func (c *consulLoader) startServicesWatch(ctx context.Context, serviceName string, tags []string, sChan chan<- []*api.ServiceEntry, watchTimeout time.Duration) error {
	if watchTimeout <= 0 {
		watchTimeout = defaultWatchTimeout
	}
	var index uint64
	qOpts := &api.QueryOptions{
		WaitIndex: index,
		WaitTime:  watchTimeout,
	}
	var err error
	// long blocking watch
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if c.cfg.Debug {
				c.logger.Printf("(re)starting watch service=%q, index=%d", serviceName, qOpts.WaitIndex)
			}
			index, err = c.watch(qOpts.WithContext(ctx), serviceName, tags, sChan)
			if err != nil {
				c.logger.Printf("service %q watch failed: %v", serviceName, err)
			}
			if index == 1 {
				qOpts.WaitIndex = index
				time.Sleep(2 * time.Second)
				continue
			}
			if index > qOpts.WaitIndex {
				qOpts.WaitIndex = index
			}
			// reset WaitIndex if the returned index decreases
			// https://www.consul.io/api-docs/features/blocking#implementation-details
			if index < qOpts.WaitIndex {
				qOpts.WaitIndex = 0
			}
		}
	}
}

func (c *consulLoader) watch(qOpts *api.QueryOptions, serviceName string, tags []string, sChan chan<- []*api.ServiceEntry) (uint64, error) {
	se, meta, err := c.client.Health().ServiceMultipleTags(serviceName, tags, true, qOpts)
	if err != nil {
		return 0, err
	}
	if meta.LastIndex == qOpts.WaitIndex {
		c.logger.Printf("service=%q did not change", serviceName)
		return meta.LastIndex, nil
	}
	if err != nil {
		return meta.LastIndex, err
	}
	if len(se) == 0 {
		return 1, nil
	}
	sChan <- se
	return meta.LastIndex, nil
}

func (c *consulLoader) serviceEntryToTargetConfig(se *api.ServiceEntry) (*types.TargetConfig, error) {
	tc := new(types.TargetConfig)
	if se.Service == nil {
		return tc, nil
	}
	for _, sd := range c.cfg.Services {
		if se.Service.Service == sd.Name {
			if sd.Config != nil {
				err := mapstructure.Decode(sd.Config, tc)
				if err != nil {
					return nil, err
				}
			}
			tc.Address = se.Service.Address
			if tc.Address == "" {
				tc.Address = se.Node.Address
			}
			tc.Address = net.JoinHostPort(tc.Address, strconv.Itoa(se.Service.Port))
			tc.Name = se.Service.ID
			return tc, nil
		}
	}
	return nil, nil
}

func (c *consulLoader) updateTargets(ctx context.Context, tcs map[string]*types.TargetConfig, opChan chan *loaders.TargetOperation) {
	targetOp, err := c.runActions(ctx, tcs, loaders.Diff(c.lastTargets, tcs))
	if err != nil {
		c.logger.Printf("failed to run actions: %v", err)
		return
	}
	numAdds := len(targetOp.Add)
	numDels := len(targetOp.Del)
	defer func() {
		consulLoaderLoadedTargets.WithLabelValues(loaderType).Set(float64(numAdds))
		consulLoaderDeletedTargets.WithLabelValues(loaderType).Set(float64(numDels))
	}()

	if numAdds+numDels == 0 {
		return
	}
	c.m.Lock()
	for _, add := range targetOp.Add {
		c.lastTargets[add.Name] = add
	}
	for _, del := range targetOp.Del {
		delete(c.lastTargets, del)
	}
	c.m.Unlock()
	opChan <- targetOp
}

//

func (c *consulLoader) readVars(ctx context.Context) error {
	if c.cfg.VarsFile == "" {
		c.vars = c.cfg.Vars
		return nil
	}
	b, err := utils.ReadFile(ctx, c.cfg.VarsFile)
	if err != nil {
		return err
	}
	v := make(map[string]interface{})
	err = yaml.Unmarshal(b, &v)
	if err != nil {
		return err
	}
	c.vars = utils.MergeMaps(v, c.cfg.Vars)
	return nil
}

func (c *consulLoader) initializeAction(cfg map[string]interface{}) (actions.Action, error) {
	if len(cfg) == 0 {
		return nil, errors.New("missing action definition")
	}
	if actType, ok := cfg["type"]; ok {
		switch actType := actType.(type) {
		case string:
			if in, ok := actions.Actions[actType]; ok {
				act := in()
				err := act.Init(cfg, actions.WithLogger(c.logger), actions.WithTargets(nil))
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

func (c *consulLoader) runActions(ctx context.Context, tcs map[string]*types.TargetConfig, targetOp *loaders.TargetOperation) (*loaders.TargetOperation, error) {
	if c.numActions == 0 {
		return targetOp, nil
	}
	var err error
	// some actions are defined
	for _, tc := range tcs {
		err = c.targetConfigFn(tc)
		if err != nil {
			c.logger.Printf("failed running target config fn on target %q", tc.Name)
		}
	}

	// run target config func and build map of targets configs
	for i, tAdd := range targetOp.Add {
		err = c.targetConfigFn(tAdd)
		if err != nil {
			return nil, err
		}
		targetOp.Add[i] = tAdd
	}

	opChan := make(chan *loaders.TargetOperation)
	doneCh := make(chan struct{})
	result := &loaders.TargetOperation{
		Add: make([]*types.TargetConfig, 0, len(targetOp.Add)),
		Del: make([]string, 0, len(targetOp.Del)),
	}
	ctx, cancel := context.WithTimeout(ctx, c.cfg.ActionsTimeout)
	defer cancel()
	// start operation gathering goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
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
			err := c.runOnAddActions(ctx, tc.Name, tcs)
			if err != nil {
				c.logger.Printf("failed running OnAdd actions: %v", err)
				return
			}
			opChan <- &loaders.TargetOperation{Add: []*types.TargetConfig{tc}}
		}(tAdd)
	}
	// run OnDelete actions
	for _, tDel := range targetOp.Del {
		go func(name string) {
			defer wg.Done()
			err := c.runOnDeleteActions(ctx, name, tcs)
			if err != nil {
				c.logger.Printf("failed running OnDelete actions: %v", err)
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

func (c *consulLoader) runOnAddActions(ctx context.Context, tName string, tcs map[string]*types.TargetConfig) error {
	aCtx := &actions.Context{
		Input:   tName,
		Env:     make(map[string]interface{}),
		Vars:    c.vars,
		Targets: tcs,
	}
	for _, act := range c.addActions {
		c.logger.Printf("running action %q for target %q", act.NName(), tName)
		res, err := act.Run(ctx, aCtx)
		if err != nil {
			// delete target from known targets map
			c.m.Lock()
			delete(c.lastTargets, tName)
			c.m.Unlock()
			return fmt.Errorf("action %q for target %q failed: %v", act.NName(), tName, err)
		}

		aCtx.Env[act.NName()] = utils.Convert(res)
		if c.cfg.Debug {
			c.logger.Printf("action %q, target %q result: %+v", act.NName(), tName, res)
			b, _ := json.MarshalIndent(aCtx, "", "  ")
			c.logger.Printf("action %q context:\n%s", act.NName(), string(b))
		}
	}
	return nil
}

func (c *consulLoader) runOnDeleteActions(ctx context.Context, tName string, tcs map[string]*types.TargetConfig) error {
	env := make(map[string]interface{})
	for _, act := range c.delActions {
		res, err := act.Run(ctx, &actions.Context{Input: tName, Env: env, Vars: c.vars})
		if err != nil {
			return fmt.Errorf("action %q for target %q failed: %v", act.NName(), tName, err)
		}
		env[act.NName()] = res
	}
	return nil
}
