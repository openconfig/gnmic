package event_to_tag

import (
	"io/ioutil"
	"log"
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

// ToTag moves ALL values matching any of the regex in .Values to the EventMsg.Tags map.
// if .Keep is true, the matching values are not deleted from EventMsg.Tags
type ToTag struct {
	Values     []string `mapstructure:"values,omitempty"`
	ValueNames []string `mapstructure:"value_names,omitempty"`
	Keep       bool     `mapstructure:"keep,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty"`
	valueNames []*regexp.Regexp
	values     []*regexp.Regexp

	logger *log.Logger
}

func init() {
	formatters.Register("event_to_tag", func() formatters.EventProcessor {
		return &ToTag{}
	})
}

func (t *ToTag) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, t)
	if err != nil {
		return err
	}
	t.valueNames = make([]*regexp.Regexp, 0, len(t.ValueNames))
	for _, reg := range t.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		t.valueNames = append(t.valueNames, re)
	}
	t.values = make([]*regexp.Regexp, 0, len(t.Values))
	for _, reg := range t.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		t.values = append(t.values, re)
	}
	if t.Debug {
		t.logger = log.New(logger.Writer(), "event_to_tag ", logger.Flags())
	} else {
		t.logger = log.New(ioutil.Discard, "", 0)
	}
	return nil
}

func (t *ToTag) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k, v := range e.Values {
		for _, re := range t.valueNames {
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
		for _, re := range t.values {
			if vs, ok := v.(string); ok {
				if re.MatchString(vs) {
					if e.Tags == nil {
						e.Tags = make(map[string]string)
					}
					e.Tags[k] = vs
					if !t.Keep {
						delete(e.Values, k)
					}
				}
			}
		}
	}
}
