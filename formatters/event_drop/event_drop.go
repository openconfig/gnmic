package event_drop

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

const (
	processorType = "event-drop"
	loggingPrefix = "[" + processorType + "] "
)

// Drop Drops the msg if ANY of the Tags or Values regexes are matched
type Drop struct {
	Condition  string   `mapstructure:"condition,omitempty"`
	TagNames   []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Tags       []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
	tags       []*regexp.Regexp
	values     []*regexp.Regexp
	code       *gojq.Code
	logger     *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Drop{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (d *Drop) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(d)
	}
	d.Condition = strings.TrimSpace(d.Condition)
	q, err := gojq.Parse(d.Condition)
	if err != nil {
		return err
	}
	d.code, err = gojq.Compile(q)
	if err != nil {
		return err
	}
	// init tag keys regex
	d.tagNames = make([]*regexp.Regexp, 0, len(d.TagNames))
	for _, reg := range d.TagNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tagNames = append(d.tagNames, re)
	}
	d.tags = make([]*regexp.Regexp, 0, len(d.Tags))
	for _, reg := range d.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tags = append(d.tags, re)
	}
	//
	d.valueNames = make([]*regexp.Regexp, 0, len(d.ValueNames))
	for _, reg := range d.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.valueNames = append(d.valueNames, re)
	}

	d.values = make([]*regexp.Regexp, 0, len(d.values))
	for _, reg := range d.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.values = append(d.values, re)
	}
	if d.logger.Writer() != io.Discard {
		b, err := json.Marshal(d)
		if err != nil {
			d.logger.Printf("initialized processor '%s': %+v", processorType, d)
			return nil
		}
		d.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (d *Drop) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		if d.Condition != "" {
			ok, err := formatters.CheckCondition(d.code, e)
			if err != nil {
				d.logger.Printf("condition check failed: %v", err)
				continue
			}
			if ok {
				*e = formatters.EventMsg{}
				continue
			}
		}
		for k, v := range e.Values {
			for _, re := range d.valueNames {
				if re.MatchString(k) {
					d.logger.Printf("value name '%s' matched regex '%s'", k, re.String())
					*e = formatters.EventMsg{}
					break
				}
			}
			for _, re := range d.values {
				if vs, ok := v.(string); ok {
					if re.MatchString(vs) {
						d.logger.Printf("value '%s' matched regex '%s'", v, re.String())
						*e = formatters.EventMsg{}
						break
					}
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range d.tagNames {
				if re.MatchString(k) {
					d.logger.Printf("tag name '%s' matched regex '%s'", k, re.String())
					*e = formatters.EventMsg{}
					break
				}
			}
			for _, re := range d.tags {
				if re.MatchString(v) {
					d.logger.Printf("tag '%s' matched regex '%s'", v, re.String())
					*e = formatters.EventMsg{}
					break
				}
			}
		}
	}
	return es
}

func (d *Drop) WithLogger(l *log.Logger) {
	if d.Debug && l != nil {
		d.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if d.Debug {
		d.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (d *Drop) WithTargets(tcs map[string]*types.TargetConfig) {}

func (d *Drop) WithActions(act map[string]map[string]interface{}) {}
