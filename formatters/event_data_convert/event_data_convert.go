package event_data_convert

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	units "github.com/bcicen/go-units"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

const (
	processorType = "event-data-convert"
	loggingPrefix = "[" + processorType + "] "
)

var stringUnitRegex = regexp.MustCompile(`([+-]?([0-9]*[.])?[0-9]+)\s?(\S+)`)

// dataConvert converts the value with key matching one of regexes, to the specified data unit
type dataConvert struct {
	Values []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	From   string   `mapstructure:"from,omitempty" json:"from,omitempty"`
	To     string   `mapstructure:"to,omitempty" json:"to,omitempty"`
	Keep   bool     `mapstructure:"keep,omitempty" json:"keep,omitempty"`
	Old    string   `mapstructure:"old,omitempty" json:"old,omitempty"`
	New    string   `mapstructure:"new,omitempty" json:"new,omitempty"`
	Debug  bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	values      []*regexp.Regexp
	renameRegex *regexp.Regexp
	logger      *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &dataConvert{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (c *dataConvert) Init(cfg interface{}, opts ...formatters.Option) error {
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
	if c.Old != "" {
		c.renameRegex, err = regexp.Compile(c.Old)
		if err != nil {
			return err
		}
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

func (c *dataConvert) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
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
					iv, err := c.convertData(k, v, nil)
					if err != nil {
						c.logger.Printf("data convert error: %v", err)
						break
					}
					c.logger.Printf("key '%s', value %v converted to %s: %f", k, v, c.To, iv)
					if c.renameRegex != nil {
						newValues[c.getNewName(k)] = iv
						if !c.Keep {
							delete(e.Values, k)
						}
						break
					}
					if c.Keep {
						newValues[fmt.Sprintf("%s_%s", k, c.To)] = iv
						break
					}
					newValues[k] = iv
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

func (c *dataConvert) WithLogger(l *log.Logger) {
	if c.Debug && l != nil {
		c.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if c.Debug {
		c.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (c *dataConvert) WithTargets(tcs map[string]*types.TargetConfig) {}

func (c *dataConvert) WithActions(act map[string]map[string]interface{}) {}

func (c *dataConvert) convertData(k string, i interface{}, from *units.Unit) (float64, error) {
	if from == nil && c.From == "" {
		from = unitFromName(k)
	}
	if from == nil {
		fr := sToU(c.From)
		from = &fr
	}
	switch i := i.(type) {
	case string:
		iv, err := strconv.Atoi(i)
		if err != nil {
			v, unit, err := parseStringUnit(i)
			if err != nil {
				return 0, err
			}
			return c.convertData(k, v, &unit)
		}
		return c.convertData(k, iv, nil)
	case int:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case int8:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case int16:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case int32:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case int64:
		if from == nil {
			*from = sToU(c.From)
		}
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case uint:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case uint8:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case uint16:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case uint32:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case uint64:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case float64:
		cv, err := units.ConvertFloat(i, *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	case float32:
		cv, err := units.ConvertFloat(float64(i), *from, sToU(c.To))
		if err != nil {
			return 0, err
		}
		return cv.Float(), nil
	default:
		return 0, fmt.Errorf("cannot convert %v, type %T", i, i)
	}
}

func sToU(s string) units.Unit {
	switch s {
	case "b":
		return units.Bit
	case "kb":
		return units.KiloBit
	case "mb":
		return units.MegaBit
	case "gb":
		return units.GigaBit
	case "tb":
		return units.TeraBit
	case "eb":
		return units.ExaBit
	//
	case "B":
		return units.Byte
	case "KB":
		return units.KiloByte
	case "MB":
		return units.MegaByte
	case "GB":
		return units.GigaByte
	case "TB":
		return units.TeraByte
	case "EB":
		return units.ExaByte
	case "ZB":
		return units.ZettaByte
	case "YB":
		return units.YottaByte
	//
	case "KiB":
		return units.Kibibyte
	case "MiB":
		return units.Mebibyte
	case "GiB":
		return units.Gibibyte
	case "TiB":
		return units.Tebibyte
	case "EiB":
		return units.Exbibyte
	case "ZiB":
		return units.Zebibyte
	case "YiB":
		return units.Yobibyte
	//
	default:
		return units.Byte
	}
}

func parseStringUnit(s string) (float64, units.Unit, error) {
	// derive unit from string
	groups := stringUnitRegex.FindAllSubmatch([]byte(s), -1)
	if len(groups) == 0 {
		return 0, units.Byte, errors.New("failed to parse string submatches")
	}
	if len(groups[0]) != 4 {
		return 0, units.Byte, errors.New("failed to parse string, unexpected number of groups")
	}
	// check if the first match is equal to the original value
	if string(groups[0][0]) != s {
		return 0, units.Byte, errors.New("failed to parse string, partial match")
	}
	f, err := strconv.ParseFloat(string(groups[0][1]), 64)
	if err != nil {
		return 0, units.Unit{}, err
	}
	return f, sToU(string(groups[0][3])), nil
}

func unitFromName(k string) *units.Unit {
	switch {
	case strings.HasSuffix(k, "_octets"), strings.HasSuffix(k, "_bytes"), strings.HasSuffix(k, "-octets"), strings.HasSuffix(k, "-bytes"):
		return &units.Byte
	}
	return nil
}

func (c *dataConvert) getNewName(k string) string {
	if c.renameRegex != nil {
		return c.renameRegex.ReplaceAllString(k, c.New)
	}
	return k
}
