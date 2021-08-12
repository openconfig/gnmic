package consul_loader

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/karimra/gnmic/loaders"
	"github.com/karimra/gnmic/types"
	"github.com/mitchellh/consulstructure"
	"github.com/mitchellh/mapstructure"
)

const (
	loggingPrefix  = "[consul_loader] "
	watchInterval  = 5 * time.Second
	defaultAddress = "localhost:8500"
	defaultPrefix  = "gnmic/config/targets"
	//
	defaultWatchTimeout = 1 * time.Minute
)

func init() {
	loaders.Register("consul", func() loaders.TargetLoader {
		return &consulLoader{
			cfg:         &cfg{},
			m:           new(sync.Mutex),
			lastTargets: make(map[string]*types.TargetConfig),
			logger:      log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

type consulLoader struct {
	cfg         *cfg
	decoder     *consulstructure.Decoder
	client      *api.Client
	m           *sync.Mutex
	lastTargets map[string]*types.TargetConfig
	logger      *log.Logger
}

type cfg struct {
	Address    string `mapstructure:"address,omitempty" json:"address,omitempty"`
	Datacenter string `mapstructure:"datacenter,omitempty" json:"datacenter,omitempty"`
	Username   string `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password   string `mapstructure:"password,omitempty" json:"password,omitempty"`
	Token      string `mapstructure:"token,omitempty" json:"token,omitempty"`

	Debug bool `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	// KV based target config loading
	KeyPrefix string `mapstructure:"key-prefix,omitempty" json:"key-prefix,omitempty"`
	// Service based target config loading
	Services []*serviceDef `mapstructure:"services,omitempty" json:"services,omitempty"`
}

type serviceDef struct {
	Name   string                 `mapstructure:"name,omitempty" json:"name,omitempty"`
	Tags   []string               `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Config map[string]interface{} `mapstructure:"config,omitempty" json:"config,omitempty"`
}

type service struct {
	ID      string
	Name    string
	Address string
	Tags    []string
}

func (c *consulLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger) error {
	err := loaders.DecodeConfig(cfg, c.cfg)
	if err != nil {
		return err
	}
	err = c.setDefaults()
	if err != nil {
		return err
	}
	if logger != nil {
		c.logger.SetOutput(logger.Writer())
		c.logger.SetFlags(logger.Flags())
	}
	c.logger.Printf("intialized consul loader: %+v", c.cfg)
	return nil
}

func (c *consulLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	opChan := make(chan *loaders.TargetOperation)

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

	if c.cfg.KeyPrefix != "" {
		updateCh := make(chan interface{})
		errCh := make(chan error)
		c.decoder = &consulstructure.Decoder{
			Target:   new(map[string]*types.TargetConfig),
			Prefix:   c.cfg.KeyPrefix,
			Consul:   clientConfig,
			UpdateCh: updateCh,
			ErrCh:    errCh,
		}
		go c.decoder.Run()

		go func() {
			c.logger.Printf("starting watch goroutine")
			defer close(opChan)
			for {
				select {
				case <-ctx.Done():
					return
				case err := <-errCh:
					c.logger.Printf("loader error: %v", err)
					continue
				case upd := <-updateCh:
					c.logger.Printf("loader update: %+v", upd)
					rs, ok := upd.(*map[string]*types.TargetConfig)
					if !ok {
						c.logger.Printf("unexpected update format: %T", upd)
						continue
					}
					for n, t := range *rs {
						if t == nil {
							t = &types.TargetConfig{
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
					op := loaders.Diff(c.lastTargets, *rs)
					c.m.Lock()
					for _, add := range op.Add {
						c.lastTargets[add.Name] = add
					}
					for _, del := range op.Del {
						delete(c.lastTargets, del)
					}
					c.m.Unlock()
					opChan <- op
				}
			}
		}()
	} else if len(c.cfg.Services) > 0 {
		var err error
	CLIENT:
		c.client, err = api.NewClient(clientConfig)
		if err != nil {
			c.logger.Printf("Failed to create a Consul client:%v", err)
			time.Sleep(2 * time.Second)
			goto CLIENT
		}
		sChan := make(chan []*service)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case srvs, ok := <-sChan:
					if !ok {
						return
					}
					c.updateTargets(srvs, opChan)
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
	}
	return opChan
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
	return nil
}

func (c *consulLoader) startServicesWatch(ctx context.Context, serviceName string, tags []string, sChan chan<- []*service, watchTimeout time.Duration) error {
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

func (c *consulLoader) watch(qOpts *api.QueryOptions, serviceName string, tags []string, sChan chan<- []*service) (uint64, error) {
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
	newSrvs := make([]*service, 0)
	for _, srv := range se {
		addr := srv.Service.Address
		if addr == "" {
			addr = srv.Node.Address
		}
		newSrvs = append(newSrvs, &service{
			ID:      srv.Service.ID,
			Name:    serviceName,
			Address: net.JoinHostPort(addr, strconv.Itoa(srv.Service.Port)),
			Tags:    srv.Service.Tags,
		})
	}
	sChan <- newSrvs
	return meta.LastIndex, nil
}

func (c *consulLoader) updateTargets(srvs []*service, opChan chan *loaders.TargetOperation) {
	tcs := make(map[string]*types.TargetConfig)
	for _, s := range srvs {
		tc := new(types.TargetConfig)
		for _, sd := range c.cfg.Services {
			if s.Name == sd.Name && sd.Config != nil {
				err := mapstructure.Decode(sd.Config, tc)
				if err != nil {
					c.logger.Printf("failed to decode config map: %v", err)
				}
			}
		}
		tc.Address = s.Address
		tc.Name = s.ID
		tcs[tc.Name] = tc
	}

	op := loaders.Diff(c.lastTargets, tcs)
	c.m.Lock()
	for _, add := range op.Add {
		c.lastTargets[add.Name] = add
	}
	for _, del := range op.Del {
		delete(c.lastTargets, del)
	}
	c.m.Unlock()
	opChan <- op
}
