package event_delete

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

const (
	processorType = "event-delete"
)

// Delete, deletes ALL the tags or values matching one of the regexes
type Delete struct {
	Tags       []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	TagNames   []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags   []*regexp.Regexp
	values []*regexp.Regexp

	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Delete{}
	})
}

func (d *Delete) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
	}
	if d.Debug && logger != nil {
		d.logger = log.New(logger.Writer(), processorType+" ", logger.Flags())
	} else if d.Debug {
		d.logger = log.New(os.Stderr, processorType+" ", log.LstdFlags|log.Lmicroseconds)
	} else {
		d.logger = log.New(ioutil.Discard, "", 0)
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
	// init tag names regex
	d.tagNames = make([]*regexp.Regexp, 0, len(d.TagNames))
	for _, reg := range d.TagNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tagNames = append(d.tagNames, re)
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
	// init values names regex
	d.valueNames = make([]*regexp.Regexp, 0, len(d.ValueNames))
	for _, reg := range d.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.valueNames = append(d.valueNames, re)
	}
	if d.logger.Writer() != ioutil.Discard {
		b, err := json.Marshal(d)
		if err != nil {
			d.logger.Printf("initialized processor '%s': %+v", processorType, d)
			return nil
		}
		d.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (d *Delete) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k, v := range e.Values {
		for _, re := range d.valueNames {
			if re.MatchString(k) {
				d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
				delete(e.Values, k)
			}
		}
		for _, re := range d.values {
			if vs, ok := v.(string); ok {
				if re.MatchString(vs) {
					d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
					delete(e.Values, k)
				}
			}
		}
	}
	for k, v := range e.Tags {
		for _, re := range d.tagNames {
			if re.MatchString(k) {
				d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
				delete(e.Tags, k)
			}
		}
		for _, re := range d.tags {
			if re.MatchString(v) {
				d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
				delete(e.Tags, k)
			}
		}
	}
}
