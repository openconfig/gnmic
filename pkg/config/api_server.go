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

	"github.com/openconfig/gnmic/pkg/api/types"
)

const (
	defaultAPIServerAddress = ":7890"
	defaultAPIServerTimeout = 10 * time.Second
	trueString              = "true"
)

type APIServer struct {
	Address               string           `mapstructure:"address,omitempty" json:"address,omitempty"`
	Timeout               time.Duration    `mapstructure:"timeout,omitempty" json:"timeout,omitempty"`
	TLS                   *types.TLSConfig `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	EnableMetrics         bool             `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	EnableProfiling       bool             `mapstructure:"enable-profiling,omitempty" json:"enable-profiling,omitempty"`
	Debug                 bool             `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	HealthzDisableLogging bool             `mapstructure:"healthz-disable-logging,omitempty" json:"healthz-disable-logging,omitempty"`
}

func (c *Config) GetAPIServer() error {
	if !c.FileConfig.IsSet("api-server") && c.API == "" {
		return nil
	}
	c.APIServer = new(APIServer)
	c.APIServer.Address = os.ExpandEnv(c.FileConfig.GetString("api-server/address"))
	if c.APIServer.Address == "" {
		c.APIServer.Address = os.ExpandEnv(c.FileConfig.GetString("api"))
	}
	c.APIServer.Timeout = c.FileConfig.GetDuration("api-server/timeout")
	if c.FileConfig.IsSet("api-server/tls") {
		c.APIServer.TLS = new(types.TLSConfig)
		c.APIServer.TLS.CaFile = os.ExpandEnv(c.FileConfig.GetString("api-server/tls/ca-file"))
		c.APIServer.TLS.CertFile = os.ExpandEnv(c.FileConfig.GetString("api-server/tls/cert-file"))
		c.APIServer.TLS.KeyFile = os.ExpandEnv(c.FileConfig.GetString("api-server/tls/key-file"))
		c.APIServer.TLS.ClientAuth = os.ExpandEnv(c.FileConfig.GetString("api-server/tls/client-auth"))
		if err := c.APIServer.TLS.Validate(); err != nil {
			return fmt.Errorf("api-server TLS config error: %w", err)
		}
	}

	c.APIServer.EnableMetrics = os.ExpandEnv(c.FileConfig.GetString("api-server/enable-metrics")) == trueString
	c.APIServer.EnableProfiling = os.ExpandEnv(c.FileConfig.GetString("api-server/enable-profiling")) == trueString
	c.APIServer.Debug = os.ExpandEnv(c.FileConfig.GetString("api-server/debug")) == trueString
	c.APIServer.HealthzDisableLogging = os.ExpandEnv(c.FileConfig.GetString("api-server/healthz-disable-logging")) == trueString
	c.setAPIServerDefaults()
	return nil
}

func (c *Config) setAPIServerDefaults() {
	if c.APIServer.Address == "" {
		c.APIServer.Address = defaultAPIServerAddress
	}
	if c.APIServer.Timeout <= 0 {
		c.APIServer.Timeout = defaultAPIServerTimeout
	}
}
