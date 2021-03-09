package consul_loader

import (
	"context"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/loaders"
	"github.com/mitchellh/consulstructure"
)

const (
	loggingPrefix  = "[consul_loader] "
	watchInterval  = 5 * time.Second
	defaultAddress = "localhost:8500"
	defaultPrefix  = "gnmic/config/targets"
)

func init() {
	loaders.Register("consul", func() loaders.TargetLoader {
		return &ConsulLoader{
			cfg:         &cfg{},
			m:           new(sync.Mutex),
			lastTargets: make(map[string]*collector.TargetConfig),
			logger:      log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

type ConsulLoader struct {
	cfg         *cfg
	decoder     *consulstructure.Decoder
	m           *sync.Mutex
	lastTargets map[string]*collector.TargetConfig
	logger      *log.Logger
}

type cfg struct {
	Address    string `mapstructure:"address,omitempty" json:"address,omitempty"`
	Datacenter string `mapstructure:"datacenter,omitempty" json:"datacenter,omitempty"`
	Username   string `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password   string `mapstructure:"password,omitempty" json:"password,omitempty"`
	Token      string `mapstructure:"token,omitempty" json:"token,omitempty"`

	KeyPrefix string `mapstructure:"key-prefix,omitempty" json:"key-prefix,omitempty"`
}

func (c *ConsulLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger) error {
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

func (c *ConsulLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
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
	updateCh := make(chan interface{})
	errCh := make(chan error)
	c.decoder = &consulstructure.Decoder{
		Target:   new(map[string]*collector.TargetConfig),
		Prefix:   c.cfg.KeyPrefix,
		Consul:   clientConfig,
		UpdateCh: updateCh,
		ErrCh:    errCh,
	}
	go c.decoder.Run()
	opChan := make(chan *loaders.TargetOperation)
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
				rs, ok := upd.(*map[string]*collector.TargetConfig)
				if !ok {
					c.logger.Printf("unexpected update format: %T", upd)
					continue
				}
				for n, t := range *rs {
					if t == nil {
						t = &collector.TargetConfig{
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
	return opChan
}

func (c *ConsulLoader) setDefaults() error {
	if c.cfg.Address == "" {
		c.cfg.Address = defaultAddress
	}
	if c.cfg.Datacenter == "" {
		c.cfg.Datacenter = "dc1"
	}
	if c.cfg.KeyPrefix == "" {
		c.cfg.KeyPrefix = defaultPrefix
	}
	return nil
}
