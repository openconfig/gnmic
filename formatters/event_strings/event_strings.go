package event_strings

import (
	"github.com/karimra/gnmic/formatters"
)

// Strings provides some of Golang's strings functions to transform, tags, tag keys, values and value keys
type Strings struct{}

func init() {
	formatters.Register("event_strings", func() formatters.EventProcessor {
		return &Strings{}
	})
}

func (r *Strings) Init(cfg interface{}) error { return nil }

func (r *Strings) Apply(e *formatters.EventMsg) {}
