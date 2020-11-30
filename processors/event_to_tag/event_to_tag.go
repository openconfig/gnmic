package event_to_tag

import (
	"regexp"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/processors"
)

type ToTag struct {
	Type   string   `mapstructure:"type,omitempty"`
	Values []string `mapstructure:"values,omitempty"`
	paths  []*regexp.Regexp
}

func (t *ToTag) Init(cfg interface{}) error {
	err := processors.DecodeConfig(cfg, t)
	if err != nil {
		return err
	}
	t.paths = make([]*regexp.Regexp, 0, len(t.Values))
	for _, reg := range t.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		t.paths = append(t.paths, re)
	}
	return nil
}

func (t *ToTag) Apply(e *formatters.EventMsg) *formatters.EventMsg {
	for k, v := range e.Values {
		for _, re := range t.paths {
			if re.MatchString(k) {
				e.Tags[k] = v.(string)
				delete(e.Values, k)
				break
			}
		}
	}
	return e
}
