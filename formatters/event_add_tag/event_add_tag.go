package event_add_tag

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

const (
	processorType = "event-add-tag"
	loggingPrefix = "[" + processorType + "] "
)

// AddTag adds a set of tags to the event message if tag
type AddTag struct {
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

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &AddTag{}
	})
}

func (p *AddTag) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	if p.Debug && logger != nil {
		p.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds)
	} else {
		p.logger = log.New(ioutil.Discard, "", 0)
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

	if p.logger.Writer() != ioutil.Discard {
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
		for k, v := range e.Values {
			for _, re := range p.valueNames {
				if re.MatchString(k) {
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
					break
				}
			}
			for _, re := range p.values {
				if vs, ok := v.(string); ok {
					if re.MatchString(vs) {
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
					break
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range p.tagNames {
				if re.MatchString(k) {
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
					break
				}
			}
			for _, re := range p.tags {
				if re.MatchString(v) {
					p.logger.Println("match", v)
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
					break
				}
			}
		}
	}
	return es
}
