package event_date_string

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

const (
	processorType = "event-date-string"
	loggingPrefix = "[" + processorType + "] "
)

// DateString converts Tags and/or Values of unix timestamp to a human readable format.
// Precision specifies the unit of the received timestamp, s, ms, us or ns.
// DateTimeFormat is the desired datetime format, it defaults to RFC3339
type DateString struct {
	Tags      []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	Values    []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Precision string   `mapstructure:"precision,omitempty" json:"precision,omitempty"`
	Format    string   `mapstructure:"format,omitempty" json:"format,omitempty"`
	Location  string   `mapstructure:"location,omitempty" json:"location,omitempty"`
	Debug     bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags     []*regexp.Regexp
	values   []*regexp.Regexp
	location *time.Location
	logger   *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &DateString{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (d *DateString) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(d)
	}
	// init values regex
	d.values = make([]*regexp.Regexp, 0, len(d.Values))
	for _, reg := range d.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.values = append(d.values, re)
	}
	// init tags regex
	d.tags = make([]*regexp.Regexp, 0, len(d.Tags))
	for _, reg := range d.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tags = append(d.tags, re)
	}
	// set tz
	d.location = time.Local
	if d.Location != "" {
		loc, err := time.LoadLocation(d.Location)
		if err != nil {
			return err
		}
		d.location = loc
	}
	if d.logger.Writer() != io.Discard {
		b, err := json.Marshal(d)
		if err != nil {
			d.logger.Printf("initialized processor '%s': %+v", processorType, d)
			return nil
		}
		d.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (d *DateString) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		for k, v := range e.Values {
			for _, re := range d.values {
				if re.MatchString(k) {
					d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
					iv, err := convertToInt(v)
					if err != nil {
						d.logger.Printf("failed to convert '%v' to date string: %v", v, err)
						continue
					}
					var td time.Time
					switch d.Precision {
					case "s", "sec", "second":
						td = time.Unix(int64(iv), 0)
					case "ms", "millisecond":
						td = time.Unix(0, int64(iv)*1000000)
					case "us", "microsecond":
						td = time.Unix(0, int64(iv)*1000)
					case "ns", "nanosecond":
						td = time.Unix(0, int64(iv))
					}
					if d.Format == "" {
						d.Format = time.RFC3339
					}
					e.Values[k] = td.In(d.location).Format(d.Format)
					break
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range d.tags {
				if re.MatchString(k) {
					d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
					iv, err := strconv.Atoi(v)
					if err != nil {
						log.Printf("failed to convert %s to int: %v", v, err)
					}
					var td time.Time
					switch d.Precision {
					case "s", "sec", "second":
						td = time.Unix(int64(iv), 0)
					case "ms", "millisecond":
						td = time.Unix(0, int64(iv)*1000000)
					case "us", "microsecond":
						td = time.Unix(0, int64(iv)*1000)
					case "ns", "nanosecond":
						td = time.Unix(0, int64(iv))
					}
					if d.Format == "" {
						d.Format = time.RFC3339
					}
					e.Values[k] = td.Format(d.Format)
					break
				}
			}
		}
	}
	return es
}

func (d *DateString) WithLogger(l *log.Logger) {
	if d.Debug && l != nil {
		d.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if d.Debug {
		d.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (d *DateString) WithTargets(tcs map[string]*types.TargetConfig) {}

func (d *DateString) WithActions(act map[string]map[string]interface{}) {}

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
