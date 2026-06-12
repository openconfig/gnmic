// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_convert

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	processorType = "event-convert"
)

// convert converts the value with key matching one of regexes, to the specified Type
type convert struct {
	formatters.BaseProcessor
	Values []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Type   string   `mapstructure:"type,omitempty" json:"type,omitempty"`
	Debug  bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	values []*regexp.Regexp
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &convert{}
	})
}

func (c *convert) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, c)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.Logger == nil {
		c.Logger = logging.DiscardLogger()
	}
	c.Logger = c.Logger.With("processor", processorType)
	c.values = make([]*regexp.Regexp, 0, len(c.Values))
	for _, reg := range c.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		c.values = append(c.values, re)
	}
	if c.Logger.Enabled(context.Background(), slog.LevelDebug) {
		if b, err := json.Marshal(c); err == nil {
			c.Logger.Debug("initialized processor", "config", string(b))
		} else {
			c.Logger.Debug("initialized processor", "config", c)
		}
	}
	return nil
}

func (c *convert) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		for k, v := range e.Values {
			for _, re := range c.values {
				if re.MatchString(k) {
					c.Logger.Debug("key matched regex", "key", k, "regex", re.String())
					switch c.Type {
					case "int":
						iv, err := convertToInt(v)
						if err != nil {
							c.Logger.Warn("convert error", "err", err)
							break
						}
						c.Logger.Debug("converted value", "key", k, "value", v, "type", c.Type, "result", iv)
						e.Values[k] = iv
					case "uint":
						iv, err := convertToUint(v)
						if err != nil {
							c.Logger.Warn("convert error", "err", err)
							break
						}
						c.Logger.Debug("converted value", "key", k, "value", v, "type", c.Type, "result", iv)
						e.Values[k] = iv
					case "string":
						iv, err := convertToString(v)
						if err != nil {
							c.Logger.Warn("convert error", "err", err)
							break
						}
						c.Logger.Debug("converted value", "key", k, "value", v, "type", c.Type, "result", iv)
						e.Values[k] = iv
					case "float":
						iv, err := convertToFloat(v)
						if err != nil {
							c.Logger.Warn("convert error", "err", err)
							break
						}
						c.Logger.Debug("converted value", "key", k, "value", v, "type", c.Type, "result", iv)
						e.Values[k] = iv
					}
					break
				}
			}
		}
	}
	return es
}

func (c *convert) WithLogger(l *slog.Logger) {
	if !c.Debug {
		l = nil
	}
	c.BaseProcessor.WithLogger(l)
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
		return 0, fmt.Errorf("cannot convert %v to int, type %T", i, i)
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
		return 0, fmt.Errorf("cannot convert %v to uint, type %T", i, i)
	}
}

func convertToFloat(i interface{}) (float64, error) {
	switch i := i.(type) {
	case []uint8:
		if len(i) == 4 {
			return float64(math.Float32frombits(binary.BigEndian.Uint32([]byte(i)))), nil
		} else if len(i) == 8 {
			return float64(math.Float64frombits(binary.BigEndian.Uint64([]byte(i)))), nil
		} else {
			return 0, nil
		}
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
		return 0, fmt.Errorf("cannot convert %v to float64, type %T", i, i)
	}
}

func convertToString(i interface{}) (string, error) {
	switch i := i.(type) {
	case string:
		return i, nil
	case int:
		return strconv.Itoa(i), nil
	case int8:
		return strconv.Itoa(int(i)), nil
	case int16:
		return strconv.Itoa(int(i)), nil
	case int32:
		return strconv.Itoa(int(i)), nil
	case int64:
		return strconv.Itoa(int(i)), nil
	case uint:
		return strconv.FormatUint(uint64(i), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(i), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(i), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(i), 10), nil
	case uint64:
		return strconv.FormatUint(uint64(i), 10), nil
	case float64:
		return strconv.FormatFloat(i, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(i), nil
	default:
		return "", fmt.Errorf("cannot convert %v to string, type %T", i, i)
	}
}
