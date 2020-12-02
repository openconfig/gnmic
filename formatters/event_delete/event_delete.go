package event_delete

import (
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

// Delete, deletes ALL the tags or values matching one of the regexes
type Delete struct {
	Tags   []string `mapstructure:"tags,omitempty"`
	Values []string `mapstructure:"values,omitempty"`

	TagKeys   []string `mapstructure:"tag_keys,omitempty"`
	ValueKeys []string `mapstructure:"value_keys,omitempty"`

	tags   []*regexp.Regexp
	values []*regexp.Regexp

	tagKeys   []*regexp.Regexp
	valueKeys []*regexp.Regexp
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
	// init tags regex
	d.tags = make([]*regexp.Regexp, 0, len(d.Tags))
	for _, reg := range d.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tags = append(d.tags, re)
	}
	// init tag keys regex
	d.tagKeys = make([]*regexp.Regexp, 0, len(d.TagKeys))
	for _, reg := range d.TagKeys {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tagKeys = append(d.tagKeys, re)
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
	// init value Keys regex
	d.valueKeys = make([]*regexp.Regexp, 0, len(d.ValueKeys))
	for _, reg := range d.ValueKeys {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.valueKeys = append(d.valueKeys, re)
	}
	return nil
}

func (d *Delete) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k, v := range e.Values {
		for _, re := range d.valueKeys {
			if re.MatchString(k) {
				delete(e.Values, k)
			}
		}
		for _, re := range d.values {
			if vs, ok := v.(string); ok {
				if re.MatchString(vs) {
					delete(e.Values, k)
				}
			}
		}
	}
	for k, v := range e.Tags {
		for _, re := range d.tagKeys {
			if re.MatchString(k) {
				delete(e.Tags, k)
			}
		}
		for _, re := range d.tags {
			if re.MatchString(v) {
				delete(e.Tags, k)
			}
		}
	}
}
