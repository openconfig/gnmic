package event_delete

import (
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

// Delete, deletes the tags or values matching one of the regexes
type Delete struct {
	Type   string
	tags   []*regexp.Regexp
	values []*regexp.Regexp
}

func (d *Delete) Apply(e *formatters.EventMsg) *formatters.EventMsg {
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
	return e
}
