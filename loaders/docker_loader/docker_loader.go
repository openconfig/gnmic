package docker_loader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dClient "github.com/docker/docker/client"
	"github.com/karimra/gnmic/actions"
	"github.com/karimra/gnmic/loaders"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"
)

const (
	loggingPrefix = "[docker_loader] "
	watchInterval = 30 * time.Second
	loaderType    = "docker"
)

func init() {
	loaders.Register(loaderType, func() loaders.TargetLoader {
		return &dockerLoader{
			cfg:         new(cfg),
			wg:          new(sync.WaitGroup),
			m:           new(sync.Mutex),
			lastTargets: make(map[string]*types.TargetConfig),
			logger:      log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type dockerLoader struct {
	cfg    *cfg
	client *dClient.Client
	wg     *sync.WaitGroup

	m              *sync.Mutex
	lastTargets    map[string]*types.TargetConfig
	targetConfigFn func(*types.TargetConfig) error
	logger         *log.Logger
	fl             []*targetFilterComp
	//
	vars          map[string]interface{}
	actionsConfig map[string]map[string]interface{}
	addActions    []actions.Action
	delActions    []actions.Action
	numActions    int
}

type targetFilterComp struct {
	fl   []filters.Args
	nt   filters.Args
	port string
	cfg  map[string]interface{}
}

type cfg struct {
	// address of docker daemon API
	Address string `json:"address,omitempty" mapstructure:"address,omitempty"`
	// interval between docker daemon queries
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`
	// timeout of docker daemon queries
	Timeout time.Duration `json:"timeout,omitempty" mapstructure:"timeout,omitempty"`
	// docker filter to apply on queried docker containers
	Filters []*targetFilter `json:"filters,omitempty" mapstructure:"filters,omitempty"`
	// time to wait before the first docker filter query
	StartDelay time.Duration `json:"start-delay,omitempty" mapstructure:"start-delay,omitempty"`
	// enable debug mode for more logging messages
	Debug bool `json:"debug,omitempty" mapstructure:"debug,omitempty"`
	// if true, registers dockerLoader prometheus metrics with the provided
	// prometheus registry
	EnableMetrics bool `json:"enable-metrics,omitempty" mapstructure:"enable-metrics,omitempty"`
	// variables definitions to be passed to the actions
	Vars map[string]interface{}
	// variable file, values in this file will be overwritten by
	// the ones defined in Vars
	VarsFile string `mapstructure:"vars-file,omitempty"`
	// list of Actions to run on new target discovery
	OnAdd []string `json:"on-add,omitempty" mapstructure:"on-add,omitempty"`
	// list of Actions to run on target removal
	OnDelete []string `json:"on-delete,omitempty" mapstructure:"on-delete,omitempty"`
}

type targetFilter struct {
	Containers []map[string]string    `json:"containers,omitempty" mapstructure:"containers,omitempty"`
	Network    map[string]string      `json:"network,omitempty" mapstructure:"network,omitempty"`
	Port       string                 `json:"port,omitempty" mapstructure:"port,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty" mapstructure:"config,omitempty"`
}

func (d *dockerLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger, opts ...loaders.Option) error {
	err := loaders.DecodeConfig(cfg, d.cfg)
	if err != nil {
		return err
	}
	d.setDefaults()
	for _, opt := range opts {
		opt(d)
	}
	d.fl = make([]*targetFilterComp, 0, len(d.cfg.Filters))
	for _, fm := range d.cfg.Filters {
		// network filter
		nflt := filters.NewArgs()
		for k, v := range fm.Network {
			nflt.Add(k, v)
		}
		// container filters
		cflt := make([]filters.Args, 0, len(fm.Containers))
		for _, sfm := range fm.Containers {
			flt := filters.NewArgs(filters.KeyValuePair{
				Key:   "status",
				Value: "running",
			})
			for k, v := range sfm {
				if strings.Contains(k, "=") {
					ks := strings.SplitN(k, "=", 2)
					flt.Add(ks[0], strings.Join(append(ks[1:], v), "="))
					continue
				}
				flt.Add(k, v)
			}
			cflt = append(cflt, flt)
		}
		// target filters
		d.fl = append(d.fl, &targetFilterComp{
			fl:   cflt,
			nt:   nflt,
			port: fm.Port,
			cfg:  fm.Config,
		})
	}

	if logger != nil {
		d.logger.SetOutput(logger.Writer())
		d.logger.SetFlags(logger.Flags())
	}

	d.client, err = d.createDockerClient()
	if err != nil {
		return err
	}

	ping, err := d.client.Ping(ctx)
	if err != nil {
		return err
	}
	err = d.readVars(ctx)
	if err != nil {
		return err
	}
	for _, actName := range d.cfg.OnAdd {
		if cfg, ok := d.actionsConfig[actName]; ok {
			fmt.Println(cfg)
			a, err := d.initializeAction(cfg)
			if err != nil {
				return err
			}
			d.addActions = append(d.addActions, a)
			continue
		}
		return fmt.Errorf("unknown action name %q", actName)

	}
	for _, actName := range d.cfg.OnDelete {
		if cfg, ok := d.actionsConfig[actName]; ok {
			a, err := d.initializeAction(cfg)
			if err != nil {
				return err
			}
			d.delActions = append(d.delActions, a)
			continue
		}
		return fmt.Errorf("unknown action name %q", actName)
	}
	d.numActions = len(d.addActions) + len(d.delActions)
	d.logger.Printf("connected to docker daemon: %+v", ping)
	d.logger.Printf("initialized loader type %q: %s", loaderType, d)
	return nil
}

func (d *dockerLoader) setDefaults() {
	if d.cfg.Interval <= 0 {
		d.cfg.Interval = watchInterval
	}
	if d.cfg.Timeout <= 0 || d.cfg.Timeout >= d.cfg.Interval {
		d.cfg.Timeout = d.cfg.Interval / 2
	}
	if len(d.cfg.Filters) == 0 {
		d.cfg.Filters = []*targetFilter{
			{
				Containers: []map[string]string{
					{"status": "running"},
				},
			},
		}
	}
}

func (d *dockerLoader) createDockerClient() (*dClient.Client, error) {
	var opts []dClient.Opt
	if d.cfg.Address == "" {
		opts = []dClient.Opt{
			dClient.FromEnv,
			dClient.WithTimeout(d.cfg.Timeout),
		}
	} else {
		opts = []dClient.Opt{
			dClient.WithAPIVersionNegotiation(),
			dClient.WithHost(d.cfg.Address),
			dClient.WithTimeout(d.cfg.Timeout),
		}
	}
	return dClient.NewClientWithOpts(opts...)
}

func (d *dockerLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	opChan := make(chan *loaders.TargetOperation)
	ticker := time.NewTicker(d.cfg.Interval)
	go func() {
		defer close(opChan)
		defer ticker.Stop()
		time.Sleep(d.cfg.StartDelay)
		// first run
		d.update(ctx, opChan)
		// periodic runs
		for {
			select {
			case <-ctx.Done():
				d.logger.Printf("%q context done: %v", loaderType, ctx.Err())
				return
			case <-ticker.C:
				d.update(ctx, opChan)
			}
		}
	}()
	return opChan
}

func (d *dockerLoader) RunOnce(ctx context.Context) (map[string]*types.TargetConfig, error) {
	d.logger.Printf("querying %q targets", loaderType)
	readTargets, err := d.getTargets(ctx)
	if err != nil {
		return nil, err
	}
	if d.cfg.Debug {
		d.logger.Printf("docker loader discovered %d target(s)", len(readTargets))
	}
	return readTargets, nil
}

// update runs the docker loader once and updates the added/remove target to the opChan
func (d *dockerLoader) update(ctx context.Context, opChan chan *loaders.TargetOperation) {
	readTargets, err := d.RunOnce(ctx)
	if err != nil {
		d.logger.Printf("failed to read targets from docker daemon: %v", err)
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
		d.updateTargets(ctx, readTargets, opChan)
	}
}

func (d *dockerLoader) getTargets(ctx context.Context) (map[string]*types.TargetConfig, error) {
	d.wg = new(sync.WaitGroup)
	d.wg.Add(len(d.fl))
	readTargets := make(map[string]*types.TargetConfig)
	m := new(sync.Mutex)
	errChan := make(chan error, len(d.fl))

	start := time.Now()
	defer dockerLoaderListRequestDuration.WithLabelValues(loaderType).
		Set(float64(time.Since(start).Nanoseconds()))

	for _, targetFilter := range d.fl {
		go func(fl *targetFilterComp) {
			dockerLoaderListRequestsTotal.WithLabelValues(loaderType).Add(1)
			defer d.wg.Done()
			// get networks
			nrs, err := d.client.NetworkList(ctx, dtypes.NetworkListOptions{
				Filters: fl.nt,
			})
			if err != nil {
				errChan <- fmt.Errorf("failed getting networks list using filter %+v: %v", fl.nt, err)
				return
			}
			// get containers for each defined filter
			for _, cfl := range fl.fl {
				conts, err := d.client.ContainerList(ctx, dtypes.ContainerListOptions{
					Filters: cfl,
				})
				if err != nil {
					errChan <- fmt.Errorf("failed getting containers list using filter %+v: %v", cfl, err)
					continue
				}
				for _, cont := range conts {
					d.logger.Printf("building target from container %q", cont.Names)
					tc := new(types.TargetConfig)
					if fl.cfg != nil {
						err = mapstructure.Decode(fl.cfg, tc)
						if err != nil {
							d.logger.Printf("failed to decode config map: %v", err)
						}
					}
					// set target name
					tc.Name = cont.ID
					if len(cont.Names) > 0 {
						tc.Name = strings.TrimLeft(cont.Names[0], "/")
					}
					// discover target address and port
					switch strings.ToLower(cont.HostConfig.NetworkMode) {
					case "host":
						if d.cfg.Address == "" || strings.HasPrefix(d.cfg.Address, "unix://") {
							tc.Address = "localhost"
						} else {
							tc.Address, _, err = net.SplitHostPort(d.cfg.Address)
							if err != nil {
								errChan <- err
								continue
							}
						}
						if fl.port != "" {
							if !strings.Contains(fl.port, "=") {
								tc.Address = fmt.Sprintf("%s:%s", tc.Address, fl.port)
							} else {
								portLabel := strings.Replace(fl.port, "label=", "", 1)
								if p, ok := cont.Labels[portLabel]; ok {
									tc.Address = fmt.Sprintf("%s:%s", tc.Address, p)
								}
							}
						}
					default:
						if strings.HasPrefix(d.cfg.Address, "unix:///") {
							for _, nr := range nrs {
								if n, ok := cont.NetworkSettings.Networks[nr.Name]; ok {
									if n.IPAddress != "" {
										tc.Address = n.IPAddress
										break
									}
									tc.Address = n.GlobalIPv6Address
									break
								}
							}
							if tc.Address == "" {
								d.logger.Printf("%q no address found", tc.Name)
								continue
							}
							if fl.port != "" {
								if !strings.Contains(fl.port, "=") {
									tc.Address = fmt.Sprintf("%s:%s", tc.Address, fl.port)
								} else {
									portLabel := strings.Replace(fl.port, "label=", "", 1)
									if p, ok := cont.Labels[portLabel]; ok {
										tc.Address = fmt.Sprintf("%s:%s", tc.Address, p)
									}
								}
							}
						} else {
							// get port from config/label
							port := getPortNumber(cont.Labels, fl.port)
							// check if port is exposed, find the public port and build the target address
							for _, p := range cont.Ports {
								// the container private port matches the port from the docker label
								if p.PrivatePort == port && p.Type == "tcp" {
									ipAddr := p.IP
									if ipAddr == "0.0.0.0" || ipAddr == "::" {
										if d.cfg.Address == "" {
											// if docker daemon is empty use localhost as target address
											ipAddr = "localhost"
										} else {
											// derive target address from daemon address if not empty
											u, err := url.Parse(d.cfg.Address)
											if err != nil {
												d.logger.Printf("failed to parse docker daemon address")
												continue
											}
											ipAddr, _, _ = net.SplitHostPort(u.Host)
										}
									}
									if ipAddr != "" && p.PublicPort != 0 {
										tc.Address = fmt.Sprintf("%s:%d", ipAddr, p.PublicPort)
									}
								}
							}
							// if an address was not found using the exposed ports
							// select the bridge address, and use the port from label if not zero
							if tc.Address == "" {
								for _, nr := range nrs {
									if n, ok := cont.NetworkSettings.Networks[nr.Name]; ok {
										if n.IPAddress != "" {
											tc.Address = n.IPAddress
											break
										}
										tc.Address = n.GlobalIPv6Address
										break
									}
								}
								if tc.Address == "" {
									d.logger.Printf("%q no address found", tc.Name)
									continue
								}
								if port != 0 {
									tc.Address = fmt.Sprintf("%s:%d", tc.Address, port)
								}
							}
						}
					}
					//
					if d.cfg.Debug {
						d.logger.Printf("discovered target config %s with filter: %v", tc, cfl)
					}
					m.Lock()
					readTargets[tc.Name] = tc
					m.Unlock()
				}
			}
		}(targetFilter)
	}
	var errors = make([]error, 0)
	go func() {
		for err := range errChan {
			errors = append(errors, err)
		}
	}()
	d.wg.Wait()
	close(errChan)
	if len(errors) > 0 {
		for _, err := range errors {
			dockerLoaderFailedListRequests.WithLabelValues(loaderType, fmt.Sprintf("%v", err)).Add(1)
			d.logger.Printf("%v", err)
		}
		return nil, fmt.Errorf("there was %d error(s)", len(errors))
	}
	return readTargets, nil
}

func (d *dockerLoader) diff(m map[string]*types.TargetConfig) *loaders.TargetOperation {
	d.m.Lock()
	defer d.m.Unlock()
	result := loaders.Diff(d.lastTargets, m)
	for _, t := range result.Add {
		if _, ok := d.lastTargets[t.Name]; !ok {
			d.lastTargets[t.Name] = t
		}
	}
	for _, n := range result.Del {
		delete(d.lastTargets, n)
	}
	dockerLoaderLoadedTargets.WithLabelValues(loaderType).Set(float64(len(result.Add)))
	dockerLoaderDeletedTargets.WithLabelValues(loaderType).Set(float64(len(result.Del)))
	if d.cfg.Debug {
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			d.logger.Printf("discovery diff result: %v", result)
		} else {
			d.logger.Printf("discovery diff result:\n%s", string(b))
		}
	}
	return result
}

func (d *dockerLoader) String() string {
	b, err := json.Marshal(d.cfg)
	if err != nil {
		return fmt.Sprintf("%+v", d.cfg)
	}
	return string(b)
}

func (d *dockerLoader) updateTargets(ctx context.Context, tcs map[string]*types.TargetConfig, opChan chan *loaders.TargetOperation) {
	var err error
	for _, tc := range tcs {
		err = d.targetConfigFn(tc)
		if err != nil {
			d.logger.Printf("failed running target config fn on target %q", tc.Name)
		}
	}
	targetOp, err := d.runActions(ctx, tcs, d.diff(tcs))
	if err != nil {
		d.logger.Printf("failed to run actions: %v", err)
		return
	}
	numAdds := len(targetOp.Add)
	numDels := len(targetOp.Del)
	defer func() {
		dockerLoaderLoadedTargets.WithLabelValues(loaderType).Set(float64(numAdds))
		dockerLoaderDeletedTargets.WithLabelValues(loaderType).Set(float64(numDels))
	}()
	if numAdds+numDels == 0 {
		return
	}
	d.m.Lock()
	for _, add := range targetOp.Add {
		d.lastTargets[add.Name] = add
	}
	for _, del := range targetOp.Del {
		delete(d.lastTargets, del)
	}
	d.m.Unlock()
	opChan <- targetOp
}

func (d *dockerLoader) readVars(ctx context.Context) error {
	if d.cfg.VarsFile == "" {
		d.vars = d.cfg.Vars
		return nil
	}
	b, err := utils.ReadFile(ctx, d.cfg.VarsFile)
	if err != nil {
		return err
	}
	v := make(map[string]interface{})
	err = yaml.Unmarshal(b, &v)
	if err != nil {
		return err
	}
	d.vars = utils.MergeMaps(v, d.cfg.Vars)
	return nil
}

func (d *dockerLoader) initializeAction(cfg map[string]interface{}) (actions.Action, error) {
	if len(cfg) == 0 {
		return nil, errors.New("missing action definition")
	}
	if actType, ok := cfg["type"]; ok {
		switch actType := actType.(type) {
		case string:
			if in, ok := actions.Actions[actType]; ok {
				act := in()
				err := act.Init(cfg, actions.WithLogger(d.logger), actions.WithTargets(nil))
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

func (d *dockerLoader) runActions(ctx context.Context, tcs map[string]*types.TargetConfig, targetOp *loaders.TargetOperation) (*loaders.TargetOperation, error) {
	if d.numActions == 0 {
		return targetOp, nil
	}
	opChan := make(chan *loaders.TargetOperation)
	// some actions are defined,
	doneCh := make(chan struct{})
	result := &loaders.TargetOperation{
		Add: make([]*types.TargetConfig, 0, len(targetOp.Add)),
		Del: make([]string, 0, len(targetOp.Del)),
	}
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
			err := d.runOnAddActions(tc.Name, tcs)
			if err != nil {
				d.logger.Printf("failed running OnAdd actions: %v", err)
				return
			}
			opChan <- &loaders.TargetOperation{Add: []*types.TargetConfig{tc}}
		}(tAdd)
	}
	// run OnDelete actions
	for _, tDel := range targetOp.Del {
		go func(name string) {
			defer wg.Done()
			err := d.runOnDeleteActions(name, tcs)
			if err != nil {
				d.logger.Printf("failed running OnDelete actions: %v", err)
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

func (d *dockerLoader) runOnAddActions(tName string, tcs map[string]*types.TargetConfig) error {
	aCtx := &actions.Context{
		Input:   tName,
		Env:     make(map[string]interface{}),
		Vars:    d.vars,
		Targets: tcs,
	}
	for _, act := range d.addActions {
		d.logger.Printf("running action %q for target %q", act.NName(), tName)
		res, err := act.Run(aCtx)
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

func (d *dockerLoader) runOnDeleteActions(tName string, tcs map[string]*types.TargetConfig) error {
	env := make(map[string]interface{})
	for _, act := range d.delActions {
		res, err := act.Run(&actions.Context{Input: tName, Env: env, Vars: d.vars})
		if err != nil {
			return fmt.Errorf("action %q for target %q failed: %v", act.NName(), tName, err)
		}
		env[act.NName()] = res
	}
	return nil
}

/// helpers

func getPortNumber(labels map[string]string, p string) uint16 {
	var port uint16
	if p != "" {
		if !strings.Contains(p, "=") {
			p, _ := strconv.Atoi(p)
			port = uint16(p)
		} else {
			s := labels[strings.Replace(p, "label=", "", 1)]
			p, _ := strconv.Atoi(s)
			port = uint16(p)
		}
	}
	return port
}
