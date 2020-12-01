package event_replace

import (
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

// Replace replaces the field names or tag names matchinging one of the regexes by the replacement string
type Replace struct {
	Type   string
	Tags   []replacer
	Values []replacer
}

type replacer struct {
	re          regexp.Regexp
	replacement string
}

func (r *Replace) Apply(e *formatters.EventMsg) *formatters.EventMsg {
	for k, v := range e.Values {
		for _, rep := range r.Values {
			nk := rep.re.ReplaceAllString(k, rep.replacement)
			if nk != k {
				delete(e.Values, k)
				e.Values[nk] = v
				break
			}
		}
	}
	for k, v := range e.Tags {
		for _, rep := range r.Tags {
			nk := rep.re.ReplaceAllString(k, rep.replacement)
			if nk != k {
				delete(e.Values, k)
				e.Tags[nk] = v
				break
			}
		}
	}
	return e
}
