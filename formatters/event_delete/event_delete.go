package event_delete

import (
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

// Delete, deletes ALL the tags or values matching one of the regexes
type Delete struct {
	Type   string   `mapstructure:"type,omitempty"`
	Tags   []string `mapstructure:"tags,omitempty"`
	Values []string `mapstructure:"values,omitempty"`
	tags   []*regexp.Regexp
	values []*regexp.Regexp
}

func init() {
	formatters.Register("event_delete", func() formatters.EventProcessor {
		return &Delete{}
	})
}

func (d *Delete) Init(cfg interface{}) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
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
	d.values = make([]*regexp.Regexp, 0, len(d.values))
	for _, reg := range d.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.values = append(d.values, re)
	}
	return nil
}

func (d *Delete) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k := range e.Values {
		for _, re := range d.values {
			if re.MatchString(k) {
				delete(e.Values, k)
			}
		}
	}
	for k := range e.Tags {
		for _, re := range d.tags {
			if re.MatchString(k) {
				delete(e.Tags, k)
			}
		}
	}
}
