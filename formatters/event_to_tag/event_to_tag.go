package event_to_tag

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

const (
	processorType = "event-to-tag"
	loggingPrefix = "[" + processorType + "] "
)

// ToTag moves ALL values matching any of the regex in .Values to the EventMsg.Tags map.
// if .Keep is true, the matching values are not deleted from EventMsg.Tags
type ToTag struct {
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Keep       bool     `mapstructure:"keep,omitempty" json:"keep,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	valueNames []*regexp.Regexp
	values     []*regexp.Regexp

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &ToTag{}
	})
}

func (t *ToTag) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, t)
	if err != nil {
		return err
	}
	if t.Debug && logger != nil {
		t.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if t.Debug {
		t.logger = log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds)
	} else {
		t.logger = log.New(ioutil.Discard, "", 0)
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
	if t.logger.Writer() != ioutil.Discard {
		b, err := json.Marshal(t)
		if err != nil {
			t.logger.Printf("initialized processor '%s': %+v", processorType, t)
			return nil
		}
		t.logger.Printf("initialized processor '%s': %s", processorType, string(b))
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
