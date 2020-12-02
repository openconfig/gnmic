package event_write

import (
	"encoding/json"
	"io"
	"os"
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

type Write struct {
	Tags   []string `mapstructure:"tags,omitempty"`
	Values []string `mapstructure:"values,omitempty"`
	Dst    string   `mapstructure:"dst,omitempty"`

	tags   []*regexp.Regexp
	values []*regexp.Regexp
	dst    io.Writer
}

func init() {
	formatters.Register("event_write", func() formatters.EventProcessor {
		return &Write{}
	})
}

func (p *Write) Init(cfg interface{}) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
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

	switch p.Dst {
	case "":
		p.dst = os.Stdout
	case "stderr":
		p.dst = os.Stderr
	default:
		p.dst, err = os.OpenFile(p.Dst, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Write) Apply(e *formatters.EventMsg) {
	if e == nil {
		p.dst.Write([]byte(""))
		return
	}
	for k := range e.Values {
		for _, re := range p.values {
			if re.MatchString(k) {
				b, err := json.Marshal(e)
				if err != nil {
					break
				}
				p.dst.Write(b)
				return
			}
		}
	}
	for k := range e.Tags {
		for _, re := range p.tags {
			if re.MatchString(k) {
				b, err := json.Marshal(e)
				if err != nil {
					break
				}
				p.dst.Write(b)
				return
			}
		}
	}
}
