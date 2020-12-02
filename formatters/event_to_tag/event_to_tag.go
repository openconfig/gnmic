package event_to_tag

import (
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

// ToTag moves ALL values matching any of the regex in .Values to the EventMsg.Tags map.
// if .Keep is true, the matching values are not deleted from EventMsg.Tags
type ToTag struct {
	Values []string `mapstructure:"values,omitempty"`
	Keep   bool     `mapstructure:"keep,omitempty"`
	values []*regexp.Regexp
}

func init() {
	formatters.Register("event_to_tag", func() formatters.EventProcessor {
		return &ToTag{}
	})
}

func (t *ToTag) Init(cfg interface{}) error {
	err := formatters.DecodeConfig(cfg, t)
	if err != nil {
		return err
	}
	t.values = make([]*regexp.Regexp, 0, len(t.Values))
	for _, reg := range t.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		t.values = append(t.values, re)
	}
	return nil
}

func (t *ToTag) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k, v := range e.Values {
		for _, re := range t.values {
			if re.MatchString(k) {
				if e.Tags == nil {
					e.Tags = make(map[string]string)
				}
				e.Tags[k] = v.(string)
				if !t.Keep {
					delete(e.Values, k)
				}
			}
		}
	}
}
