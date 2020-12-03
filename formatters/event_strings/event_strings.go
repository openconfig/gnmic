package event_strings

import (
	"github.com/karimra/gnmic/formatters"
)

// TODO

// Strings provides some of Golang's strings functions to transform: tags, tag keys, values and value keys
type Strings struct {
	Tags   []string `mapstructure:"tags,omitempty"`
	Values []string `mapstructure:"values,omitempty"`

	TagKeys   []string `mapstructure:"tag_keys,omitempty"`
	ValueKeys []string `mapstructure:"value_keys,omitempty"`
}

func init() {
	formatters.Register("event_strings", func() formatters.EventProcessor {
		return &Strings{}
	})
}

func (r *Strings) Init(cfg interface{}) error { return nil }

func (r *Strings) Apply(e *formatters.EventMsg) {}
