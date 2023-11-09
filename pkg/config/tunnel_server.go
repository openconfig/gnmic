// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/openconfig/gnmic/pkg/types"
	"github.com/openconfig/gnmic/pkg/utils"
)

const (
	defaultTargetWaitTime = 2 * time.Second
)

type tunnelServer struct {
	Address string `mapstructure:"address,omitempty" json:"address,omitempty"`
	// TLS
	TLS *types.TLSConfig `mapstructure:"tls,omitempty"`
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

	if c.FileConfig.IsSet("tunnel-server/tls") {
		c.TunnelServer.TLS = new(types.TLSConfig)
		c.TunnelServer.TLS.CaFile = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/tls/ca-file"))
		c.TunnelServer.TLS.CertFile = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/tls/cert-file"))
		c.TunnelServer.TLS.KeyFile = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/tls/key-file"))
		c.TunnelServer.TLS.ClientAuth = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/tls/client-auth"))
		if err := c.TunnelServer.TLS.Validate(); err != nil {
			return fmt.Errorf("tunnel-server TLS config error: %w", err)
		}
	}
	c.TunnelServer.TargetWaitTime = c.FileConfig.GetDuration("tunnel-server/target-wait-time")
	c.TunnelServer.EnableMetrics = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/enable-metrics")) == trueString
	c.TunnelServer.Debug = os.ExpandEnv(c.FileConfig.GetString("tunnel-server/debug")) == trueString

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
