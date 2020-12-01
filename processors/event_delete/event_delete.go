package event_delete

import (
	"log"
	"regexp"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/processors"
)

// Delete, deletes the tags or values matching one of the regexes
type Delete struct {
	Type   string   `json:"type,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Values []string `json:"values,omitempty"`
	tags   []*regexp.Regexp
	values []*regexp.Regexp
}

func init() {
	processors.Register("event_delete", func() processors.EventProcessor {
		return &Delete{
			Type: "event_delete",
		}
	})
}

func (d *Delete) Init(cfg interface{}) error {
	err := processors.DecodeConfig(cfg, d)
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
		log.Printf("regex: %s is  %+v", reg, re)
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
				break
			}
		}
	}
	for k := range e.Tags {
		for _, re := range d.tags {
			if re.MatchString(k) {
				delete(e.Tags, k)
				break
			}
		}
	}
}
