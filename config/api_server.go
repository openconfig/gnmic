package config

import (
	"os"
	"time"
)

const (
	defaultAPIServerAddress = ":7890"
	defaultAPIServerTimeout = 10 * time.Second
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
	c.APIServer.SkipVerify = os.ExpandEnv(c.FileConfig.GetString("api-server/skip-verify")) == "true"
	c.APIServer.CaFile = os.ExpandEnv(c.FileConfig.GetString("api-server/ca-file"))
	c.APIServer.CertFile = os.ExpandEnv(c.FileConfig.GetString("api-server/cert-file"))
	c.APIServer.KeyFile = os.ExpandEnv(c.FileConfig.GetString("api-server/key-file"))

	c.APIServer.EnableMetrics = os.ExpandEnv(c.FileConfig.GetString("api-server/enable-metrics")) == "true"
	c.APIServer.Debug = os.ExpandEnv(c.FileConfig.GetString("api-server/debug")) == "true"
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
