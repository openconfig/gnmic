package event_to_tag

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"regexp"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
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
		return &ToTag{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (t *ToTag) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, t)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(t)
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
	if t.logger.Writer() != io.Discard {
		b, err := json.Marshal(t)
		if err != nil {
			t.logger.Printf("initialized processor '%s': %+v", processorType, t)
			return nil
		}
		t.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (t *ToTag) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
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
	return es
}

func (t *ToTag) WithLogger(l *log.Logger) {
	if t.Debug && l != nil {
		t.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if t.Debug {
		t.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (t *ToTag) WithTargets(tcs map[string]*types.TargetConfig) {}

func (t *ToTag) WithActions(act map[string]map[string]interface{}) {}
