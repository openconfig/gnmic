package config

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/karimra/gnmic/outputs"
	_ "github.com/karimra/gnmic/outputs/all"
)

func (c *Config) GetOutputs() (map[string]map[string]interface{}, error) {
	outDef := c.FileConfig.GetStringMap("outputs")
	if len(outDef) == 0 && !c.FileConfig.GetBool("subscribe-quiet") {
		stdoutConfig := map[string]interface{}{
			"type":      "file",
			"file-type": "stdout",
			"format":    c.FileConfig.GetString("format"),
		}
		outDef["default-stdout"] = stdoutConfig
	}
	for name, outputCfg := range outDef {
		outputCfgconv := convert(outputCfg)
		switch outCfg := outputCfgconv.(type) {
		case map[string]interface{}:
			if outType, ok := outCfg["type"]; ok {
				if _, ok := outputs.OutputTypes[outType.(string)]; !ok {
					return nil, fmt.Errorf("unknown output type: %q", outType)
				}
				if _, ok := outputs.Outputs[outType.(string)]; ok {
					format, ok := outCfg["format"]
					if !ok || (ok && format == "") {
						outCfg["format"] = c.FileConfig.GetString("format")
					}
					c.Outputs[name] = outCfg
					continue
				}
				c.logger.Printf("unknown output type '%s'", outType)
				continue
			}
			c.logger.Printf("missing output 'type' under %v", outCfg)
		default:
			c.logger.Printf("unknown configuration format expecting a map[string]interface{}: got %T : %v", outCfg, outCfg)
		}
	}
	for n := range c.Outputs {
		expandMapEnv(c.Outputs[n], "msg-template", "target-template")
	}
	namedOutputs := c.FileConfig.GetStringSlice("subscribe-output")
	if len(namedOutputs) == 0 {
		if c.Debug {
			c.logger.Printf("outputs: %+v", c.Outputs)
		}
		return c.Outputs, nil
	}
	filteredOutputs := make(map[string]map[string]interface{})
	notFound := make([]string, 0)
	for _, name := range namedOutputs {
		if o, ok := c.Outputs[name]; ok {
			filteredOutputs[name] = o
		} else {
			notFound = append(notFound, name)
		}
	}
	if len(notFound) > 0 {
		return nil, fmt.Errorf("named output(s) not found in config file: %v", notFound)
	}
	if c.Debug {
		c.logger.Printf("outputs: %+v", filteredOutputs)
	}
	return filteredOutputs, nil
}

func convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		nm := map[string]interface{}{}
		for k, v := range x {
			nm[k.(string)] = convert(v)
		}
		return nm
	case map[string]interface{}:
		for k, v := range x {
			x[k] = convert(v)
		}
	case []interface{}:
		for i, v := range x {
			x[i] = convert(v)
		}
	}
	return i
}

type outputSuggestion struct {
	Name  string
	Types []string
}

func (c *Config) GetOutputsSuggestions() []outputSuggestion {
	outDef := c.FileConfig.GetStringMap("outputs")
	suggestions := make([]outputSuggestion, 0, len(outDef))
	for name, d := range outDef {
		dl := convert(d)
		sug := outputSuggestion{Name: name, Types: make([]string, 0)}
		switch outs := dl.(type) {
		case []interface{}:
			for _, ou := range outs {
				switch ou := ou.(type) {
				case map[string]interface{}:
					if outType, ok := ou["type"]; ok {
						sug.Types = append(sug.Types, outType.(string))
					}
				}
			}
		}
		suggestions = append(suggestions, sug)
	}
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Name < suggestions[j].Name
	})
	return suggestions
}

func (c *Config) GetOutputsConfigs() [][]string {
	outDef := c.FileConfig.GetStringMap("outputs")
	if outDef == nil {
		return nil
	}
	outList := make([][]string, 0, len(outDef))
	for name, outputCfg := range outDef {
		b, err := json.Marshal(outputCfg)
		if err != nil {
			c.logger.Printf("could not marshal output config: %v", err)
			return nil
		}
		outList = append(outList, []string{name, string(b)})
	}
	return outList
}
