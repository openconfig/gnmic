package event_convert

import (
	"regexp"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/processors"
)

// Convert converts the value with path matching one of regexes to the specified Type
type Convert struct {
	Type       string   `mapstructure:"type,omitempty"`
	Values     []string `mapstructure:"values,omitempty"`
	TargetUnit string   `mapstructure:"target_unit,omitempty"`
	values     []*regexp.Regexp
}

func init() {
	processors.Register("event_convert", func() processors.EventProcessor {
		return &Convert{
			Type: "event_convert",
		}
	})
}

func (c *Convert) Init(cfg interface{}) error {
	err := processors.DecodeConfig(cfg, c)
	if err != nil {
		return err
	}
	c.values = make([]*regexp.Regexp, 0, len(c.Values))
	for _, reg := range c.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		c.values = append(c.values, re)
	}
	return nil
}

func (c *Convert) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k, v := range e.Values {
		for _, re := range c.values {
			if re.MatchString(k) {
				switch c.TargetUnit {
				case "int":
					if iv, ok := v.(int); ok {
						e.Values[k] = iv
					}
				case "uint":
					if iv, ok := v.(uint); ok {
						e.Values[k] = iv
					}
				case "string":
					if iv, ok := v.(string); ok {
						e.Values[k] = iv
					}
				case "float":
					if iv, ok := v.(float64); ok {
						e.Values[k] = iv
					}
				}
				break
			}
		}
	}
}
