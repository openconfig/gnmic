package config

import (
	"fmt"

	"github.com/karimra/gnmic/inputs"
	_ "github.com/karimra/gnmic/inputs/all"
)

func (c *Config) GetInputs() (map[string]map[string]interface{}, error) {
	errs := make([]error, 0)
	inputsDef := c.FileConfig.GetStringMap("inputs")
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
					c.Inputs[name] = inputCfg
					continue
				}
				err := fmt.Errorf("unknown input type '%s'", outType)
				c.logger.Print(err)
				errs = append(errs, err)
				continue
			}
			err := fmt.Errorf("missing input 'type' under %v", inputCfg)
			c.logger.Print(err)
			errs = append(errs, err)
		default:
			c.logger.Printf("unknown configuration format expecting a map[string]interface{}: got %T : %v", inputCfg, inputCfg)
			return nil, fmt.Errorf("unexpected inputs configuration format")
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("there was %d error(s) when getting inputs configuration", len(errs))
	}
	for n := range c.Inputs {
		expandMapEnv(c.Inputs[n])
	}
	if c.Debug {
		c.logger.Printf("inputs: %+v", c.Inputs)
	}
	return c.Inputs, nil
}
