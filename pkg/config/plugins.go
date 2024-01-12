package config

import (
	"time"
)

type PluginsConfig struct {
	Path         string        `mapstructure:"path,omitempty" json:"path,omitempty"`
	Glob         string        `mapstructure:"glob,omitempty" json:"glob,omitempty"`
	StartTimeout time.Duration `mapstructure:"start-timeout,omitempty" json:"start-timeout,omitempty"`
	Debug        bool          `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

func (c *Config) GetPluginsConfig() (*PluginsConfig, error) {
	if !c.FileConfig.IsSet("plugins") && c.GlobalFlags.PluginProcessorsPath == "" {
		return nil, nil
	}
	pc := &PluginsConfig{}
	pc.Path = c.GlobalFlags.PluginProcessorsPath
	if pc.Path == "" {
		pc.Path = c.FileConfig.GetString("plugins/path")
	}
	pc.Glob = c.FileConfig.GetString("plugins/glob")
	if pc.Glob == "" {
		pc.Glob = "*"
	}
	pc.StartTimeout = c.FileConfig.GetDuration("plugins/start-timeout")
	pc.Debug = c.FileConfig.GetBool("plugins/debug")
	return pc, nil
}
