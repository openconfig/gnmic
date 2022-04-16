package event_data_convert

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

const (
	processorType = "event-duration-convert"
	loggingPrefix = "[" + processorType + "] "
)

var durationRegex = regexp.MustCompile(`((?P<weeks>\d+)w)?((?P<days>\d+)d)?((?P<hours>\d+)h)?((?P<minutes>\d+)m)?((?P<seconds>\d+)s)?`)

// durationConvert converts the value with key matching one of regexes, to the specified duration precision
type durationConvert struct {
	Values []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Keep   bool     `mapstructure:"keep,omitempty" json:"keep,omitempty"`
	Debug  bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	values []*regexp.Regexp
	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &durationConvert{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (c *durationConvert) Init(cfg interface{}, opts ...formatters.Option) error {
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
	if c.logger.Writer() != io.Discard {
		b, err := json.Marshal(c)
		if err != nil {
			c.logger.Printf("initialized processor '%s': %+v", processorType, c)
			return nil
		}
		c.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}

	return nil
}

func (c *durationConvert) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		// add new Values to a new map to avoid multiple chained regex matches
		newValues := make(map[string]interface{})
		for k, v := range e.Values {
			for _, re := range c.values {
				if re.MatchString(k) {
					c.logger.Printf("key '%s' matched regex '%s'", k, re.String())
					dur, err := c.convertDuration(k, v)
					if err != nil {
						c.logger.Printf("duration convert error: %v", err)
						break
					}
					c.logger.Printf("key '%s', value %v converted to seconds: %d", k, v, dur)
					if c.Keep {
						newValues[fmt.Sprintf("%s_seconds", k)] = dur
						break
					}
					newValues[k] = dur
					break
				}
			}
		}
		// add new values to the original message
		for k, v := range newValues {
			e.Values[k] = v
		}
	}
	return es
}

func (c *durationConvert) WithLogger(l *log.Logger) {
	if c.Debug && l != nil {
		c.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if c.Debug {
		c.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (c *durationConvert) WithTargets(tcs map[string]*types.TargetConfig) {}

func (c *durationConvert) WithActions(act map[string]map[string]interface{}) {}

func (c *durationConvert) convertDuration(k string, i interface{}) (int64, error) {
	switch i := i.(type) {
	case string:
		iv, err := strconv.Atoi(i)
		if err != nil {
			return parseStringDuration(i)
		}
		return c.convertDuration(k, iv)
	case int:
		return int64(i), nil
	case int8:
		return int64(i), nil
	case int16:
		return int64(i), nil
	case int32:
		return int64(i), nil
	case int64:
		return int64(i), nil
	case uint:
		return int64(i), nil
	case uint8:
		return int64(i), nil
	case uint16:
		return int64(i), nil
	case uint32:
		return int64(i), nil
	case uint64:
		return int64(i), nil
	case float64:
		return int64(i), nil
	case float32:
		return int64(i), nil
	default:
		return 0, fmt.Errorf("cannot convert %v, type %T", i, i)
	}
}

func parseStringDuration(s string) (int64, error) {
	match := durationRegex.FindStringSubmatch(s)
	namedGroups := make(map[string]string)
	for i, name := range durationRegex.SubexpNames() {
		if i != 0 && name != "" {
			namedGroups[name] = match[i]
		}
	}
	r := int64(0)
	for k, v := range namedGroups {
		if v == "" {
			continue
		}
		switch k {
		case "weeks":
			i, err := strconv.Atoi(v)
			if err != nil {
				return 0, err
			}
			r += int64(i) * 7 * 24 * 60 * 60
		case "days":
			i, err := strconv.Atoi(v)
			if err != nil {
				return 0, err
			}
			r += int64(i) * 24 * 60 * 60
		case "hours":
			i, err := strconv.Atoi(v)
			if err != nil {
				return 0, err
			}
			r += int64(i) * 60 * 60
		case "minutes":
			i, err := strconv.Atoi(v)
			if err != nil {
				return 0, err
			}
			r += int64(i) * 60
		case "seconds":
			i, err := strconv.Atoi(v)
			if err != nil {
				return 0, err
			}
			r += int64(i)
		}
	}
	return r, nil
}
