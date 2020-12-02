package event_drop

import (
	"fmt"
	"os"
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

// Drop Drops the msg if ANY of the Tags or Values regexes are matched
type Drop struct {
	Tags   []string
	Values []string

	tags   []*regexp.Regexp
	values []*regexp.Regexp
}

func init() {
	formatters.Register("event_drop", func() formatters.EventProcessor {
		return &Drop{}
	})
}

func (d *Drop) Init(cfg interface{}) error {
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

func (d *Drop) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k := range e.Values {
		for _, re := range d.values {
			if re.MatchString(k) {
				fmt.Fprintf(os.Stdout, "matched %s\n", k)
				*e = formatters.EventMsg{}
				return
			}
		}
	}
	for k := range e.Tags {
		for _, re := range d.tags {
			if re.MatchString(k) {
				*e = formatters.EventMsg{}
				return
			}
		}
	}
}
