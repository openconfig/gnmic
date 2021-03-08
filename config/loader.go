package config

import _ "github.com/karimra/gnmic/loaders/all"

func (c *Config) GetLoader() (map[string]interface{}, error) {
	return c.FileConfig.GetStringMap("loader"), nil
}
