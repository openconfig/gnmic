package event_convert

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"

	"github.com/karimra/gnmic/formatters"
)

const (
	processorType = "event-convert"
	loggingPrefix = "[" + processorType + "] "
)

// Convert converts the value with key matching one of regexes, to the specified Type
type Convert struct {
	Values []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Type   string   `mapstructure:"type,omitempty" json:"type,omitempty"`
	Debug  bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	values []*regexp.Regexp
	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Convert{
			logger: log.New(ioutil.Discard, "", 0),
		}
	})
}

func (c *Convert) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, c)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(c)
	}
	c.values = make([]*regexp.Regexp, 0, len(c.Values))
	for _, reg := range c.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		c.values = append(c.values, re)
	}
	if c.logger.Writer() != ioutil.Discard {
		b, err := json.Marshal(c)
		if err != nil {
			c.logger.Printf("initialized processor '%s': %+v", processorType, c)
			return nil
		}
		c.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (c *Convert) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		for k, v := range e.Values {
			for _, re := range c.values {
				if re.MatchString(k) {
					c.logger.Printf("key '%s' matched regex '%s'", k, re.String())
					switch c.Type {
					case "int":
						iv, err := convertToInt(v)
						if err != nil {
							c.logger.Printf("convert error: %v", err)
							break
						}
						c.logger.Printf("key '%s', value %v converted to %s: %d", k, v, c.Type, iv)
						e.Values[k] = iv
					case "uint":
						iv, err := convertToUint(v)
						if err != nil {
							c.logger.Printf("convert error: %v", err)
							break
						}
						c.logger.Printf("key '%s', value %v converted to %s: %d", k, v, c.Type, iv)
						e.Values[k] = iv
					case "string":
						iv, err := convertToString(v)
						if err != nil {
							c.logger.Printf("convert error: %v", err)
							break
						}
						c.logger.Printf("key '%s', value %v converted to %s: %s", k, v, c.Type, iv)
						e.Values[k] = iv
					case "float":
						iv, err := convertToFloat(v)
						if err != nil {
							c.logger.Printf("convert error: %v", err)
							break
						}
						c.logger.Printf("key '%s', value %v converted to %s: %f", k, v, c.Type, iv)
						e.Values[k] = iv
					}
					break
				}
			}
		}
	}
	return es
}

func (c *Convert) WithLogger(l *log.Logger) {
	if c.Debug && l != nil {
		c.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if c.Debug {
		c.logger = log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds)
	}
}

func (c *Convert) WithTargets(tcs map[string]interface{}) {}

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
	case int8:
		return int(i), nil
	case int16:
		return int(i), nil
	case int32:
		return int(i), nil
	case int64:
		return int(i), nil
	case uint:
		return int(i), nil
	case uint8:
		return int(i), nil
	case uint16:
		return int(i), nil
	case uint32:
		return int(i), nil
	case uint64:
		return int(i), nil
	case float64:
		return int(i), nil
	case float32:
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
		if i < 0 {
			return 0, nil
		}
		return uint(i), nil
	case int8:
		if i < 0 {
			return 0, nil
		}
		return uint(i), nil
	case int16:
		if i < 0 {
			return 0, nil
		}
		return uint(i), nil
	case int32:
		if i < 0 {
			return 0, nil
		}
		return uint(i), nil
	case int64:
		if i < 0 {
			return 0, nil
		}
		return uint(i), nil
	case uint:
		return i, nil
	case uint8:
		return uint(i), nil
	case uint16:
		return uint(i), nil
	case uint32:
		return uint(i), nil
	case uint64:
		return uint(i), nil
	case float32:
		if i < 0 {
			return 0, nil
		}
		return uint(i), nil
	case float64:
		if i < 0 {
			return 0, nil
		}
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
	case int8:
		return float64(i), nil
	case int16:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case uint8:
		return float64(i), nil
	case uint16:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint64:
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
