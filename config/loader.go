package config

import (
	"errors"
	"fmt"

	"github.com/karimra/gnmic/loaders"
	_ "github.com/karimra/gnmic/loaders/all"
)

func (c *Config) GetLoader() error {
	if c.GlobalFlags.TargetsFile != "" {
		c.Loader = map[string]interface{}{
			"type": "file",
			"path": c.GlobalFlags.TargetsFile,
		}
		return nil
	}

	c.Loader = c.FileConfig.GetStringMap("loader")
	for k, v := range c.Loader {
		c.Loader[k] = convert(v)
	}

	if len(c.Loader) == 0 {
		return nil
	}
	if _, ok := c.Loader["type"]; !ok {
		return errors.New("missing type field under loader configuration")
	}
	if lds, ok := c.Loader["type"].(string); ok {
		for _, lt := range loaders.LoadersTypes {
			if lt == lds {
				expandMapEnv(c.Loader)
				return nil
			}
		}
		return fmt.Errorf("unknown loader type %q", lds)
	}
	return fmt.Errorf("field 'type' not a string, found a %T", c.Loader["type"])

}
