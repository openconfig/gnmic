package event_date_string

import (
	"errors"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/karimra/gnmic/formatters"
)

// DateString converts Tags and/or Values of unix timestamp to a human readable format.
// Precision specifies the unit of the received timestamp, s, ms, us or ns.
// DateTimeFormat is the desired datetime format, it defaults to RFC3339
type DateString struct {
	Tags               []string `mapstructure:"tag_names,omitempty"`
	Values             []string `mapstructure:"value_names,omitempty"`
	TimestampPrecision string   `mapstructure:"timestamp_precision,omitempty"`
	DateTimeFormat     string   `mapstructure:"date_time_format,omitempty"`
	Location           string   `mapstructure:"location,omitempty"`
	Debug              bool     `mapstructure:"debug,omitempty"`

	tags     []*regexp.Regexp
	values   []*regexp.Regexp
	location *time.Location
	logger   *log.Logger
}

func init() {
	formatters.Register("event_date_string", func() formatters.EventProcessor {
		return &DateString{}
	})
}

func (d *DateString) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
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
	if d.Debug {
		d.logger = log.New(logger.Writer(), "event_date_string ", logger.Flags())
	} else {
		d.logger = log.New(ioutil.Discard, "", 0)
	}
	return nil
}

func (d *DateString) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
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
				switch d.TimestampPrecision {
				case "s", "sec", "second":
					td = time.Unix(int64(iv), 0)
				case "ms", "millisecond":
					td = time.Unix(0, int64(iv)*1000000)
				case "us", "microsecond":
					td = time.Unix(0, int64(iv)*1000)
				case "ns", "nanosecond":
					td = time.Unix(0, int64(iv))
				}
				if d.DateTimeFormat == "" {
					d.DateTimeFormat = time.RFC3339
				}
				e.Values[k] = td.In(d.location).Format(d.DateTimeFormat)
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
				switch d.TimestampPrecision {
				case "s", "sec", "second":
					td = time.Unix(int64(iv), 0)
				case "ms", "millisecond":
					td = time.Unix(0, int64(iv)*1000000)
				case "us", "microsecond":
					td = time.Unix(0, int64(iv)*1000)
				case "ns", "nanosecond":
					td = time.Unix(0, int64(iv))
				}
				if d.DateTimeFormat == "" {
					d.DateTimeFormat = time.RFC3339
				}
				e.Values[k] = td.Format(d.DateTimeFormat)
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
