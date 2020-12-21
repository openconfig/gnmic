package config

import (
	"fmt"
	"sort"

	"github.com/karimra/gnmic/outputs"
	"github.com/spf13/viper"
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
	outputsConfigs := make(map[string]map[string]interface{})
	for name, outputCfg := range outDef {
		outputCfgconv := convert(outputCfg)
		switch outCfg := outputCfgconv.(type) {
		case map[string]interface{}:
			if outType, ok := outCfg["type"]; ok {
				if _, ok := outputs.Outputs[outType.(string)]; ok {
					format, ok := outCfg["format"]
					if !ok || (ok && format == "") {
						outCfg["format"] = c.FileConfig.GetString("format")
					}
					outputsConfigs[name] = outCfg
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

	namedOutputs := c.FileConfig.GetStringSlice("subscribe-output")
	if len(namedOutputs) == 0 {
		return outputsConfigs, nil
	}
	filteredOutputs := make(map[string]map[string]interface{})
	notFound := make([]string, 0)
	for _, name := range namedOutputs {
		if o, ok := outputsConfigs[name]; ok {
			filteredOutputs[name] = o
		} else {
			notFound = append(notFound, name)
		}
	}
	if len(notFound) > 0 {
		return nil, fmt.Errorf("named output(s) not found in config file: %v", notFound)
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
	outDef := viper.GetStringMap("outputs")
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
