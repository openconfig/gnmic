package config

import (
	"fmt"

	"github.com/karimra/gnmic/inputs"
	_ "github.com/karimra/gnmic/inputs/all"
)

func (c *Config) GetInputs() (map[string]map[string]interface{}, error) {
	inputsDef := c.FileConfig.GetStringMap("inputs")
	inputsConfigs := make(map[string]map[string]interface{})
	for name, inputCfg := range inputsDef {
		inputCfgconv := convert(inputCfg)
		switch inputCfg := inputCfgconv.(type) {
		case map[string]interface{}:
			if outType, ok := inputCfg["type"]; ok {
				if !strInlist(outType.(string), inputs.InputTypes) {
					return nil, fmt.Errorf("unknown output type: %q", outType)
				}
				if _, ok := inputs.Inputs[outType.(string)]; ok {
					format, ok := inputCfg["format"]
					if !ok || (ok && format == "") {
						inputCfg["format"] = c.FileConfig.GetString("format")
					}
					inputsConfigs[name] = inputCfg
					continue
				}
				c.logger.Printf("unknown input type '%s'", outType)
				continue
			}
			c.logger.Printf("missing input 'type' under %v", inputCfg)
		default:
			c.logger.Printf("unknown configuration format expecting a map[string]interface{}: got %T : %v", inputCfg, inputCfg)
		}
	}

	if c.Globals.Debug {
		c.logger.Printf("inputs: %+v", inputsConfigs)
	}
	return inputsConfigs, nil
}
