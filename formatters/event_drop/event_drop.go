package event_drop

import (
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

// Drop Drops the msg if ANY of the Tags or Values regexes are matched
type Drop struct {
	Tags   []*regexp.Regexp
	Values []*regexp.Regexp
}

func init() {
	formatters.Register("event_drop", func() formatters.EventProcessor {
		return &Drop{}
	})
}

func (d *Drop) Init(cfg interface{}) error { return nil }

func (d *Drop) Apply(e *formatters.EventMsg) {
	for k := range e.Values {
		for _, re := range d.Values {
			if re.MatchString(k) {
				d = nil
				return
			}
		}
	}
	for k := range e.Tags {
		for _, re := range d.Tags {
			if re.MatchString(k) {
				d = nil
				return
			}
		}
	}
}
