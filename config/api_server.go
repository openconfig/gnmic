// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"time"
)

const (
	defaultAPIServerAddress = ":7890"
	defaultAPIServerTimeout = 10 * time.Second
	trueString              = "true"
)

type APIServer struct {
	Address string        `mapstructure:"address,omitempty" json:"address,omitempty"`
	Timeout time.Duration `mapstructure:"timeout,omitempty" json:"timeout,omitempty"`
	// TLS
	SkipVerify bool   `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty"`
	CaFile     string `mapstructure:"ca-file,omitempty" json:"ca-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty" json:"cert-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty" json:"key-file,omitempty"`
	//
	EnableMetrics bool `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	Debug         bool `mapstructure:"debug,omitempty" json:"debug,omitempty"`
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
	c.APIServer.SkipVerify = os.ExpandEnv(c.FileConfig.GetString("api-server/skip-verify")) == trueString
	c.APIServer.CaFile = os.ExpandEnv(c.FileConfig.GetString("api-server/ca-file"))
	c.APIServer.CertFile = os.ExpandEnv(c.FileConfig.GetString("api-server/cert-file"))
	c.APIServer.KeyFile = os.ExpandEnv(c.FileConfig.GetString("api-server/key-file"))

	c.APIServer.EnableMetrics = os.ExpandEnv(c.FileConfig.GetString("api-server/enable-metrics")) == trueString
	c.APIServer.Debug = os.ExpandEnv(c.FileConfig.GetString("api-server/debug")) == trueString
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
