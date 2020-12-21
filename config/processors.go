package config

import "fmt"

func (c *Config) GetEventProcessors() (map[string]map[string]interface{}, error) {
	eps := c.FileConfig.GetStringMap("processors")
	evpConfig := make(map[string]map[string]interface{})
	for name, epc := range eps {
		switch epc := epc.(type) {
		case map[string]interface{}:
			evpConfig[name] = epc
		case nil:
			return nil, nil
		default:
			c.logger.Printf("malformed processors config, %+v", epc)
			return nil, fmt.Errorf("malformed processors config, got %T", epc)
		}
	}
	return evpConfig, nil
}
