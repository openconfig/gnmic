package event_date_string

import (
	"errors"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/karimra/gnmic/formatters"
)

// DateString converts Tags and/or Values of unix timestamp to a human readable format.
// Unit specifies the unit of the received timestamp, s, ms, us or ns.
// DateTimeFormat is the desired datetime format, it defaults to RFC3339
type DateString struct {
	Type            string   `mapstructure:"type,omitempty"`
	Tags            []string `mapstructure:"tags,omitempty"`
	Values          []string `mapstructure:"values,omitempty"`
	TimestampFormat string   `mapstructure:"timestamp_format,omitempty"`
	DateTimeFormat  string   `mapstructure:"date_time_format,omitempty"`
	Location        string   `mapstructure:"location,omitempty"`

	tags     []*regexp.Regexp
	values   []*regexp.Regexp
	location *time.Location
}

func init() {
	formatters.Register("event_date_string", func() formatters.EventProcessor {
		return &DateString{Type: "event_date_string"}
	})
}

func (d *DateString) Init(cfg interface{}) error {
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
	return nil
}

func (d *DateString) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k, v := range e.Values {
		for _, re := range d.values {
			if re.MatchString(k) {
				iv, err := convertToInt(v)
				if err != nil {
					log.Printf("failed to convert '%v' to timestamp: %v", v, err)
					continue
				}
				var td time.Time
				switch d.TimestampFormat {
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
				iv, err := strconv.Atoi(v)
				if err != nil {
					log.Printf("failed to convert %s to int: %v", v, err)
				}
				var td time.Time
				switch d.TimestampFormat {
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
