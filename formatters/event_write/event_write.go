package event_write

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
	processorType = "event-write"
	loggingPrefix = "[" + processorType + "] "
)

type Write struct {
	Condition  string   `mapstructure:"condition,omitempty"`
	Tags       []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	TagNames   []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Dst        string   `mapstructure:"dst,omitempty" json:"dst,omitempty"`
	Separator  string   `mapstructure:"separator,omitempty" json:"separator,omitempty"`
	Indent     string   `mapstructure:"indent,omitempty" json:"indent,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags       []*regexp.Regexp
	values     []*regexp.Regexp
	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
	dst        io.Writer
	sep        []byte
	code       *gojq.Code
	logger     *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Write{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *Write) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}
	p.Condition = strings.TrimSpace(p.Condition)
	q, err := gojq.Parse(p.Condition)
	if err != nil {
		return err
	}
	p.code, err = gojq.Compile(q)
	if err != nil {
		return err
	}
	if p.Separator == "" {
		p.sep = []byte("\n")
	} else {
		p.sep = []byte(p.Separator)
	}
	p.tags = make([]*regexp.Regexp, 0, len(p.Tags))
	for _, reg := range p.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.tags = append(p.tags, re)
	}
	//
	p.values = make([]*regexp.Regexp, 0, len(p.values))
	for _, reg := range p.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.values = append(p.values, re)
	}
	//
	p.tagNames = make([]*regexp.Regexp, 0, len(p.TagNames))
	for _, reg := range p.TagNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.tagNames = append(p.tagNames, re)
	}
	//
	p.valueNames = make([]*regexp.Regexp, 0, len(p.ValueNames))
	for _, reg := range p.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.valueNames = append(p.valueNames, re)
	}
	switch p.Dst {
	case "", "stdout":
		p.dst = os.Stdout
	case "stderr":
		p.dst = os.Stderr
	default:
		p.dst, err = os.OpenFile(p.Dst, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
	}

	b, err := json.Marshal(p)
	if err != nil {
		p.logger.Printf("initialized processor '%s': %+v", processorType, p)
		return nil
	}
	p.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	return nil
}

func (p *Write) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
OUTER:
	for _, e := range es {
		if e == nil {
			p.dst.Write([]byte(""))
			continue
		}
		if p.code != nil {
			ok, err := formatters.CheckCondition(p.code, e)
			if err != nil {
				p.logger.Printf("condition check failed: %v", err)
			}
			if ok {
				err := p.write(e)
				if err != nil {
					p.logger.Printf("failed to write to destination: %v", err)
					continue OUTER
				}
			}
		}
		for k, v := range e.Values {
			for _, re := range p.values {
				if vs, ok := v.(string); ok {
					if re.MatchString(vs) {
						err := p.write(e)
						if err != nil {
							p.logger.Printf("failed to write to destination: %v", err)
							continue OUTER
						}
						continue OUTER
					}
				}
			}
			for _, re := range p.valueNames {
				if re.MatchString(k) {
					err := p.write(e)
					if err != nil {
						p.logger.Printf("failed to write to destination: %v", err)
						continue OUTER
					}
					continue OUTER
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range p.tagNames {
				if re.MatchString(k) {
					err := p.write(e)
					if err != nil {
						p.logger.Printf("failed to write to destination: %v", err)
						continue OUTER
					}
					continue OUTER
				}
			}
			for _, re := range p.tags {
				if re.MatchString(v) {
					err := p.write(e)
					if err != nil {
						p.logger.Printf("failed to write to destination: %v", err)
						continue OUTER
					}
					continue OUTER
				}
			}
		}
	}
	return es
}

func (p *Write) WithLogger(l *log.Logger) {
	if p.Debug && l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *Write) write(e *formatters.EventMsg) error {
	var b []byte
	var err error
	if len(p.Indent) > 0 {
		b, err = json.MarshalIndent(e, "", p.Indent)
		if err != nil {
			return err
		}
	} else {
		b, err = json.Marshal(e)
		if err != nil {
			return err
		}
	}
	p.dst.Write(append(b, p.sep...))
	return nil
}

func (p *Write) WithTargets(tcs map[string]*types.TargetConfig) {}

func (p *Write) WithActions(act map[string]map[string]interface{}) {}
