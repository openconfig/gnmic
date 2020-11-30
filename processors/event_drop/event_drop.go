package event_drop

import (
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

// Drop Drops the msg if any of the Tags or Values regexes are matched
type Drop struct {
	Type   string
	Tags   []*regexp.Regexp
	Values []*regexp.Regexp
}

func (d *Drop) Apply(e *formatters.EventMsg) *formatters.EventMsg {
	for k := range e.Values {
		for _, re := range d.Values {
			if re.MatchString(k) {
				return nil
			}
		}
	}
	for k := range e.Tags {
		for _, re := range d.Tags {
			if re.MatchString(k) {
				return nil
			}
		}
	}
	return e
}
