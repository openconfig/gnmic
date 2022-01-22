package event_add_tag

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
	processorType = "event-add-tag"
	loggingPrefix = "[" + processorType + "] "
)

// AddTag adds a set of tags to the event message if tag
type AddTag struct {
	Condition  string            `mapstructure:"condition,omitempty"`
	Tags       []string          `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string          `mapstructure:"values,omitempty" json:"values,omitempty"`
	TagNames   []string          `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string          `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Overwrite  bool              `mapstructure:"overwrite,omitempty" json:"overwrite,omitempty"`
	Add        map[string]string `mapstructure:"add,omitempty" json:"add,omitempty"`
	Debug      bool              `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags       []*regexp.Regexp
	values     []*regexp.Regexp
	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
	code       *gojq.Code
	logger     *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &AddTag{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *AddTag) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.Condition != "" {
		p.Condition = strings.TrimSpace(p.Condition)
		q, err := gojq.Parse(p.Condition)
		if err != nil {
			return err
		}
		p.code, err = gojq.Compile(q)
		if err != nil {
			return err
		}
	}
	// init tags regex
	p.tags = make([]*regexp.Regexp, 0, len(p.Tags))
	for _, reg := range p.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.tags = append(p.tags, re)
	}
	// init tag names regex
	p.tagNames = make([]*regexp.Regexp, 0, len(p.TagNames))
	for _, reg := range p.TagNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.tagNames = append(p.tagNames, re)
	}
	// init values regex
	p.values = make([]*regexp.Regexp, 0, len(p.Values))
	for _, reg := range p.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.values = append(p.values, re)
	}
	// init value names regex
	p.valueNames = make([]*regexp.Regexp, 0, len(p.ValueNames))
	for _, reg := range p.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.valueNames = append(p.valueNames, re)
	}

	if p.logger.Writer() != io.Discard {
		b, err := json.Marshal(p)
		if err != nil {
			p.logger.Printf("initialized processor '%s': %+v", processorType, p)
			return nil
		}
		p.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (p *AddTag) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		// condition is set
		if p.code != nil && p.Condition != "" {
			ok, err := formatters.CheckCondition(p.code, e)
			if err != nil {
				p.logger.Printf("condition check failed: %v", err)
			}
			if ok {
				p.addTags(e)
			}
			continue
		}
		// no condition, check regexes
		for k, v := range e.Values {
			for _, re := range p.valueNames {
				if re.MatchString(k) {
					p.addTags(e)
					break
				}
			}
			for _, re := range p.values {
				if vs, ok := v.(string); ok {
					if re.MatchString(vs) {
						p.addTags(e)
					}
					break
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range p.tagNames {
				if re.MatchString(k) {
					p.addTags(e)
					break
				}
			}
			for _, re := range p.tags {
				if re.MatchString(v) {
					p.addTags(e)
					break
				}
			}
		}
	}
	return es
}

func (p *AddTag) WithLogger(l *log.Logger) {
	if p.Debug && l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *AddTag) WithTargets(tcs map[string]*types.TargetConfig) {}

func (p *AddTag) WithActions(act map[string]map[string]interface{}) {}

func (p *AddTag) addTags(e *formatters.EventMsg) {
	if e.Tags == nil {
		e.Tags = make(map[string]string)
	}
	for nk, nv := range p.Add {
		if p.Overwrite {
			e.Tags[nk] = nv
			continue
		}
		if _, ok := e.Tags[nk]; !ok {
			e.Tags[nk] = nv
		}
	}
}
