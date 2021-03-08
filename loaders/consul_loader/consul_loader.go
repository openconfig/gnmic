package consul_loader

import (
	"context"
	"io/ioutil"
	"log"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/loaders"
)

const (
	loggingPrefix = "[consul_loader] "
	watchInterval = 5 * time.Second
)

func init() {
	loaders.Register("file", func() loaders.TargetLoader {
		return &ConsulLoader{
			cfg:    &cfg{},
			logger: log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

type ConsulLoader struct {
	cfg         *cfg
	lastTargets map[string]*collector.TargetConfig
	logger      *log.Logger
}

type cfg struct {
	Address    string `mapstructure:"address,omitempty" json:"address,omitempty"`
	Datacenter string `mapstructure:"datacenter,omitempty" json:"datacenter,omitempty"`
	Username   string `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password   string `mapstructure:"password,omitempty" json:"password,omitempty"`
	Token      string `mapstructure:"token,omitempty" json:"token,omitempty"`
}

func (c *ConsulLoader) Init(ctx context.Context, cfg map[string]interface{}) error
func (c *ConsulLoader) Start(ctx context.Context) chan *loaders.TargetOperation
