package docker_loader

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dClient "github.com/docker/docker/client"
	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/loaders"
	"github.com/mitchellh/mapstructure"
)

const (
	loggingPrefix = "[docker_loader] "
	watchInterval = 30 * time.Second
	loaderName    = "docker"
)

func init() {
	loaders.Register(loaderName, func() loaders.TargetLoader {
		return &dockerLoader{
			cfg:         new(cfg),
			wg:          new(sync.WaitGroup),
			lastTargets: make(map[string]*collector.TargetConfig),
			logger:      log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

type dockerLoader struct {
	cfg         *cfg
	client      *dClient.Client
	wg          *sync.WaitGroup
	lastTargets map[string]*collector.TargetConfig
	logger      *log.Logger
	fl          []*targetFilterComp
}

type targetFilterComp struct {
	fl   []filters.Args
	nt   filters.Args
	port string
	cfg  map[string]interface{}
}

type cfg struct {
	Address  string          `json:"address,omitempty" mapstructure:"address,omitempty"`
	Interval time.Duration   `json:"interval,omitempty" mapstructure:"interval,omitempty"`
	Timeout  time.Duration   `json:"timeout,omitempty" mapstructure:"timeout,omitempty"`
	Filters  []*targetFilter `json:"filters,omitempty" mapstructure:"filters,omitempty"`
	Debug    bool            `json:"debug,omitempty" mapstructure:"debug,omitempty"`
}

type targetFilter struct {
	Containers []map[string]string    `json:"containers,omitempty" mapstructure:"containers,omitempty"`
	Network    map[string]string      `json:"network,omitempty" mapstructure:"network,omitempty"`
	Port       string                 `json:"port,omitempty" mapstructure:"port,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty" mapstructure:"config,omitempty"`
}

func (d *dockerLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger) error {
	err := loaders.DecodeConfig(cfg, d.cfg)
	if err != nil {
		return err
	}
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

	d.client, err = d.createDockerClient()
	if err != nil {
		return err
	}
	if logger != nil {
		d.logger.SetOutput(logger.Writer())
		d.logger.SetFlags(logger.Flags())
	}
	d.logger.Printf("initialized loader type %q: %s", loaderName, d)
	return nil
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
	go func() {
		defer close(opChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				d.logger.Printf("querying %q targets", loaderName)
				readTargets, err := d.getTargets(ctx)
				if _, ok := err.(*os.PathError); ok {
					time.Sleep(d.cfg.Interval)
					continue
				}
				if err != nil {
					d.logger.Printf("failed to read targets from docker daemon: %v", err)
					time.Sleep(d.cfg.Interval)
					continue
				}
				select {
				case <-ctx.Done():
					return
				case opChan <- d.diff(readTargets):
					time.Sleep(d.cfg.Interval)
				}
			}
		}
	}()
	return opChan
}

func (d *dockerLoader) getTargets(ctx context.Context) (map[string]*collector.TargetConfig, error) {
	d.wg = new(sync.WaitGroup)
	d.wg.Add(len(d.fl))
	readTargets := make(map[string]*collector.TargetConfig)
	m := new(sync.Mutex)
	errChan := make(chan error, len(d.fl))
	for _, targetFilter := range d.fl {
		go func(fl *targetFilterComp) {
			defer d.wg.Done()
			// get networks
			nrs, err := d.client.NetworkList(ctx, types.NetworkListOptions{
				Filters: fl.nt,
			})
			if err != nil {
				errChan <- fmt.Errorf("failed getting networks list using filter %+v: %v", fl.nt, err)
				return
			}

			for _, cfl := range fl.fl {
				conts, err := d.client.ContainerList(ctx, types.ContainerListOptions{
					Filters: cfl,
				})
				if err != nil {
					errChan <- fmt.Errorf("failed getting containers list using filter %+v: %v", cfl, err)
					return
				}
				for _, cont := range conts {
					name := cont.ID
					if len(cont.Names) > 0 {
						name = strings.TrimLeft(cont.Names[0], "/")
					}
					if d.cfg.Debug {
						d.logger.Printf("filter %v returned container %v", cfl, name)
					}
					tc := new(collector.TargetConfig)
					if fl.cfg != nil {
						err = mapstructure.Decode(fl.cfg, tc)
						if err != nil {
							d.logger.Printf("failed to decode config map: %v", err)
						}
						if d.cfg.Debug {
							d.logger.Printf("target config before adding name and address: %v", tc)
						}
					}
					tc.Name = name
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
					}
					//
					if d.cfg.Debug {
						d.logger.Printf("discoved target config %s with filter: %v", tc, cfl)
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
			d.logger.Printf("%v", err)
		}
		return nil, fmt.Errorf("there was %d error(s)", len(errors))
	}
	return readTargets, nil
}

func (d *dockerLoader) diff(m map[string]*collector.TargetConfig) *loaders.TargetOperation {
	result := loaders.Diff(d.lastTargets, m)
	for _, t := range result.Add {
		if _, ok := d.lastTargets[t.Name]; !ok {
			d.lastTargets[t.Name] = t
		}
	}
	for _, n := range result.Del {
		delete(d.lastTargets, n)
	}
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
