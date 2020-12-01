package event_convert

import (
	"errors"
	"log"
	"regexp"
	"strconv"

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
					iv, err := convertToInt(v)
					if err != nil {
						log.Printf("convert errors: %v", err)
						return
					}
					e.Values[k] = iv
				case "uint":
					iv, err := convertToUint(v)
					if err != nil {
						log.Printf("convert errors: %v", err)
						return
					}
					e.Values[k] = iv
				case "string":
					iv, err := convertToString(v)
					if err != nil {
						log.Printf("convert errors: %v", err)
						return
					}
					e.Values[k] = iv
				case "float":
					iv, err := convertToFloat(v)
					if err != nil {
						log.Printf("convert errors: %v", err)
						return
					}
					e.Values[k] = iv
				}
				break
			}
		}
	}
}

func convertToInt(i interface{}) (int, error) {
	switch i := i.(type) {
	case string:
		iv, err := strconv.Atoi(i)
		if err != nil {
			return 0, err
		}
		return iv, nil
	case int:
		return i, nil
	case uint:
		return int(i), nil
	case float64:
		return int(i), nil
	default:
		return 0, errors.New("cannot convert to int")
	}
}

func convertToUint(i interface{}) (uint, error) {
	switch i := i.(type) {
	case string:
		iv, err := strconv.Atoi(i)
		if err != nil {
			return 0, err
		}
		return uint(iv), nil
	case int:
		return 0, nil
	case uint:
		return i, nil
	case float64:
		return uint(i), nil
	default:
		return 0, errors.New("cannot convert to uint")
	}
}

func convertToFloat(i interface{}) (float64, error) {
	switch i := i.(type) {
	case string:
		iv, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return 0, err
		}
		return iv, nil
	case int:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case float64:
		return i, nil
	default:
		return 0, errors.New("cannot convert to float64")
	}
}

func convertToString(i interface{}) (string, error) {
	switch i := i.(type) {
	case string:
		return i, nil
	case int:
		return strconv.Itoa(i), nil
	case uint:
		return strconv.Itoa(int(i)), nil
	case float64:
		return strconv.FormatFloat(i, 'f', 64, 64), nil
	default:
		return "", errors.New("cannot convert to float64")
	}
}
