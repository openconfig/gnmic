package config

import (
	"errors"
	"fmt"

	"github.com/karimra/gnmic/loaders"
	_ "github.com/karimra/gnmic/loaders/all"
)

func (c *Config) GetLoader() (map[string]interface{}, error) {
	if c.GlobalFlags.TargetsFile != "" {
		return map[string]interface{}{
			"type": "file",
			"file": c.GlobalFlags.TargetsFile,
		}, nil
	}
	ldCfg := c.FileConfig.GetStringMap("loader")
	if len(ldCfg) == 0 {
		return nil, nil
	}
	if _, ok := ldCfg["type"]; !ok {
		return nil, errors.New("missing type field under loader configuration")
	}
	if lds, ok := ldCfg["type"].(string); ok {
		for _, lt := range loaders.LoadersTypes {
			if lt == lds {
				expandMapEnv(ldCfg)
				return ldCfg, nil
			}
		}
		return nil, fmt.Errorf("unknown loader type %q", lds)
	}
	return nil, fmt.Errorf("field 'type' not a string, found a %T", ldCfg["type"])

}
