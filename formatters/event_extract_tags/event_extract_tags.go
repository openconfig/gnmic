package event_extract_tags

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
	processorType = "event-extract-tags"
	loggingPrefix = "[" + processorType + "] "
)

// extractTags extracts tags from a value, a value name, a tag name or a tag value using regex named groups
type extractTags struct {
	Tags       []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	TagNames   []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Overwrite  bool     `mapstructure:"overwrite,omitempty" json:"overwrite,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags       []*regexp.Regexp
	values     []*regexp.Regexp
	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
	logger     *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &extractTags{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *extractTags) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
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

func (p *extractTags) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		for k, v := range e.Values {
			for _, re := range p.valueNames {
				p.addTags(e, re, k)
			}
			for _, re := range p.values {
				if vs, ok := v.(string); ok {
					p.addTags(e, re, vs)
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range p.tagNames {
				p.addTags(e, re, k)
			}
			for _, re := range p.tags {
				p.addTags(e, re, v)
			}
		}
	}
	return es
}

func (p *extractTags) WithLogger(l *log.Logger) {
	if p.Debug && l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *extractTags) WithTargets(tcs map[string]*types.TargetConfig) {}

func (p *extractTags) WithActions(act map[string]map[string]interface{}) {}

func (p *extractTags) addTags(e *formatters.EventMsg, re *regexp.Regexp, s string) {
	if e.Tags == nil {
		e.Tags = make(map[string]string)
	}

	matches := re.FindStringSubmatch(s)
	if p.Debug {
		p.logger.Printf("matches: %+v", matches)
	}
	if len(matches) != len(re.SubexpNames()) {
		return
	}
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			if p.Debug {
				p.logger.Printf("adding: name=%s, value=%s", name, matches[i])
			}
			if p.Overwrite {
				e.Tags[name] = matches[i]
				continue
			}
			if _, ok := e.Tags[matches[i]]; !ok {
				e.Tags[name] = matches[i]
			}
		}
	}
}
