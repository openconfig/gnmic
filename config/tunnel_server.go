package config

import (
	"fmt"
	"os"
	"time"

	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/mitchellh/mapstructure"
)

const (
	defaultTargetWaitTime = 2 * time.Second
)

type tunnelServer struct {
	Address string `mapstructure:"address,omitempty" json:"address,omitempty"`
	// TLS
	SkipVerify bool   `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty"`
	CaFile     string `mapstructure:"ca-file,omitempty" json:"ca-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty" json:"cert-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty" json:"key-file,omitempty"`
	//
	TargetWaitTime time.Duration `mapstructure:"target-wait-time,omitempty" json:"target-wait-time,omitempty"`
	//
	EnableMetrics bool `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	Debug         bool `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	// targets
	Targets []*targetMatch `mapstructure:"targets,omitempty" json:"targets,omitempty"`
}

type targetMatch struct {
	// target Type as reported by the tunnel.Target to the Tunnel Server
	Type string `mapstructure:"type,omitempty" json:"type,omitempty"`
	// a Regex pattern to check the target ID as reported by
	// the tunnel.Target to the Tunnel Server
	ID string `mapstructure:"id,omitempty" json:"id,omitempty"`
	// Optional gnmic.Target Configuration that will be assigned to the target with
	// an ID matching the above regex
	Config types.TargetConfig `mapstructure:"config,omitempty" json:"config,omitempty"`
}

func (c *Config) GetTunnelServer() error {
	if !c.FileConfig.IsSet("tunnel-server") {
		return nil
	}
	c.TunnelServer = new(tunnelServer)
	c.TunnelServer.Address = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/address"))
	c.TunnelServer.SkipVerify = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/skip-verify")) == "true"
	c.TunnelServer.CaFile = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/ca-file"))
	c.TunnelServer.CertFile = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/cert-file"))
	c.TunnelServer.KeyFile = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/key-file"))
	c.TunnelServer.TargetWaitTime = c.FileConfig.GetDuration("tunnel-server/target-wait-time")
	c.TunnelServer.EnableMetrics = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/enable-metrics")) == "true"
	c.TunnelServer.Debug = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/debug")) == "true"

	var err error
	c.TunnelServer.Targets = make([]*targetMatch, 0)
	targetMatches := c.FileConfig.Get("tunnel-server/targets")
	switch targetMatches := targetMatches.(type) {
	case []interface{}:
		for _, tmi := range targetMatches {
			tm := new(targetMatch)
			err = mapstructure.Decode(utils.Convert(tmi), tm)
			if err != nil {
				return err
			}
			c.TunnelServer.Targets = append(c.TunnelServer.Targets, tm)
		}
	case nil:
	default:
		return fmt.Errorf("tunnel-server has an unexpected target configuration type %T", targetMatches)
	}

	c.setTunnelServerDefaults()
	return nil
}

func (c *Config) setTunnelServerDefaults() {
	if c.TunnelServer.Address == "" {
		c.TunnelServer.Address = defaultAddress
	}
	if c.TunnelServer.TargetWaitTime <= 0 {
		c.TunnelServer.TargetWaitTime = defaultTargetWaitTime
	}
}
