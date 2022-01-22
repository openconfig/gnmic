package event_merge

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

const (
	processorType = "event-merge"
	loggingPrefix = "[" + processorType + "] "
)

// Merge merges a list of event messages into one or multiple messages based on some criteria
type Merge struct {
	Always bool `mapstructure:"always,omitempty" json:"always,omitempty"`
	Debug  bool `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Merge{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *Merge) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
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

func (p *Merge) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	if len(es) == 0 {
		return nil
	}
	if p.Always {
		for i, e := range es {
			if e == nil {
				continue
			}
			if i > 0 {
				merge(es[0], e)
			}
		}
		return []*formatters.EventMsg{es[0]}
	}
	result := make([]*formatters.EventMsg, 0, len(es))
	timestamps := make(map[int64]int)
	for _, e := range es {
		if e == nil {
			continue
		}
		if idx, ok := timestamps[e.Timestamp]; ok {
			merge(result[idx], e)
			continue
		}
		result = append(result, e)
		timestamps[e.Timestamp] = len(result) - 1
	}
	return result
}

func (p *Merge) WithLogger(l *log.Logger) {
	if p.Debug && l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *Merge) WithTargets(tcs map[string]*types.TargetConfig) {}

func (p *Merge) WithActions(act map[string]map[string]interface{}) {}

func merge(e1, e2 *formatters.EventMsg) {
	if e1.Tags == nil {
		e1.Tags = make(map[string]string)
	}
	if e1.Values == nil {
		e1.Values = make(map[string]interface{})
	}
	for n, t := range e2.Tags {
		e1.Tags[n] = t
	}
	for n, v := range e2.Values {
		e1.Values[n] = v
	}
	e1.Deletes = append(e1.Deletes, e2.Deletes...)
	if e2.Timestamp > e1.Timestamp {
		e1.Timestamp = e2.Timestamp
	}
}
