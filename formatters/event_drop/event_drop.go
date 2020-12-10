package event_drop

import (
	"io/ioutil"
	"log"
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

const (
	processorType = "event-drop"
)

// Drop Drops the msg if ANY of the Tags or Values regexes are matched
type Drop struct {
	TagNames   []string `mapstructure:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty"`
	Tags       []string `mapstructure:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty"`

	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
	tags       []*regexp.Regexp
	values     []*regexp.Regexp

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Drop{}
	})
}

func (d *Drop) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
	}
	// init tag keys regex
	d.tagNames = make([]*regexp.Regexp, 0, len(d.TagNames))
	for _, reg := range d.TagNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tagNames = append(d.tagNames, re)
	}
	d.tags = make([]*regexp.Regexp, 0, len(d.Tags))
	for _, reg := range d.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tags = append(d.tags, re)
	}
	//
	d.valueNames = make([]*regexp.Regexp, 0, len(d.ValueNames))
	for _, reg := range d.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.valueNames = append(d.valueNames, re)
	}

	d.values = make([]*regexp.Regexp, 0, len(d.values))
	for _, reg := range d.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.values = append(d.values, re)
	}
	if d.Debug {
		d.logger = log.New(logger.Writer(), processorType+" ", logger.Flags())
	} else {
		d.logger = log.New(ioutil.Discard, "", 0)
	}
	return nil
}

func (d *Drop) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k, v := range e.Values {
		for _, re := range d.valueNames {
			if re.MatchString(k) {
				d.logger.Printf("value name '%s' matched regex '%s'", k, re.String())
				*e = formatters.EventMsg{}
				return
			}
		}
		for _, re := range d.values {
			if vs, ok := v.(string); ok {
				if re.MatchString(vs) {
					d.logger.Printf("value '%s' matched regex '%s'", v, re.String())
					*e = formatters.EventMsg{}
					return
				}
			}
		}
	}
	for k, v := range e.Tags {
		for _, re := range d.tagNames {
			if re.MatchString(k) {
				d.logger.Printf("tag name '%s' matched regex '%s'", k, re.String())
				*e = formatters.EventMsg{}
				return
			}
		}
		for _, re := range d.tags {
			if re.MatchString(v) {
				d.logger.Printf("tag '%s' matched regex '%s'", v, re.String())
				*e = formatters.EventMsg{}
				return
			}
		}
	}
}
