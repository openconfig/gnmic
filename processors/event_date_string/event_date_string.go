package event_date_string

import (
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/processors"
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

	tags   []*regexp.Regexp
	values []*regexp.Regexp
}

func (d *DateString) Init(cfg interface{}) error {
	err := processors.DecodeConfig(cfg, d)
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
	return nil
}

func (d *DateString) Apply(e *formatters.EventMsg) *formatters.EventMsg {
	for k, v := range e.Values {
		for _, re := range d.values {
			if re.MatchString(k) {
				if iv, ok := v.(float64); ok {
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
				}
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
	return e
}
