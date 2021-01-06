package config

import (
	"fmt"

	"github.com/karimra/gnmic/formatters"
)

func (c *Config) GetEventProcessors() (map[string]map[string]interface{}, error) {
	eps := c.FileConfig.GetStringMap("processors")
	evpConfig := make(map[string]map[string]interface{})
	for name, epc := range eps {
		switch epc := epc.(type) {
		case map[string]interface{}:
			c.logger.Printf("validating processor %q config", name)
			err := c.validateProcessorConfig(epc)
			if err != nil {
				return nil, err
			}
			evpConfig[name] = epc
		case nil:
			return nil, fmt.Errorf("empty processor %q config", name)
		default:
			c.logger.Printf("malformed processors config, %+v", epc)
			return nil, fmt.Errorf("malformed processors config, got %T", epc)
		}
	}
	if c.Globals.Debug {
		c.logger.Printf("processors: %+v", evpConfig)
	}
	return evpConfig, nil
}

func (c *Config) validateProcessorConfig(pcfg map[string]interface{}) error {
	for epType := range pcfg {
		if !strInlist(epType, formatters.EventProcessorTypes) {
			return fmt.Errorf("unknown processors type: %s", epType)
		}
	}
	return nil
}

func strInlist(s string, ls []string) bool {
	for _, ss := range ls {
		if ss == s {
			return true
		}
	}
	return false
}
