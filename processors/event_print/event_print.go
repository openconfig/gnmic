package event_print

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

type Print struct {
	Type   string
	Tags   []regexp.Regexp
	Values []regexp.Regexp
	Dst    string
	dst    io.Writer
}

func (p *Print) Apply(e *formatters.EventMsg) *formatters.EventMsg {
	for k := range e.Values {
		for _, re := range p.Values {
			if re.MatchString(k) {
				b, err := json.Marshal(e)
				if err != nil {
					break
				}
				fmt.Fprint(p.dst, string(b))
			}
		}
	}
	for k := range e.Tags {
		for _, re := range p.Tags {
			if re.MatchString(k) {
				b, err := json.Marshal(e)
				if err != nil {
					break
				}
				fmt.Fprint(p.dst, string(b))
			}
		}
	}
	return e
}
