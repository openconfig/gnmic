package event_print

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/karimra/gnmic/formatters"
)

type Print struct {
	Type   string   `mapstructure:"type,omitempty"`
	Tags   []string `mapstructure:"tags,omitempty"`
	Values []string `mapstructure:"values,omitempty"`
	Dst    string   `mapstructure:"dst,omitempty"`

	tags   []*regexp.Regexp
	values []*regexp.Regexp
	dst    io.Writer
}

func init() {
	formatters.Register("event_print", func() formatters.EventProcessor {
		return &Print{
			//Type: "event_print",
		}
	})
}

func (p *Print) Init(cfg interface{}) error {
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
	case "stderr":
		p.dst = os.Stderr
	default:
		p.dst = os.Stdout
	}
	return nil
}

func (p *Print) Apply(e *formatters.EventMsg) {
	for k := range e.Values {
		for _, re := range p.values {
			if re.MatchString(k) {
				b, err := json.Marshal(e)
				if err != nil {
					break
				}
				fmt.Fprintf(p.dst, "%s\n", string(b))
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
				fmt.Fprintf(p.dst, "%s\n", string(b))
				return
			}
		}
	}
}
