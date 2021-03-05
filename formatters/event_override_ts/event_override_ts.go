package event_override_ts

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/karimra/gnmic/formatters"
)

const (
	processorType = "event-override-ts"
	loggingPrefix = "[" + processorType + "] "
)

// OverrideTS Overrides the message timestamp with the local time
type OverrideTS struct {
	Precision string `mapstructure:"precision,omitempty" json:"precision,omitempty"`
	Debug     bool   `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &OverrideTS{}
	})
}

func (o *OverrideTS) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, 0)
	if err != nil {
		return err
	}
	if o.Debug && logger != nil {
		o.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if o.Debug {
		o.logger = log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds)
	} else {
		o.logger = log.New(ioutil.Discard, "", 0)
	}
	if o.Precision == "" {
		o.Precision = "ns"
	}
	if o.logger.Writer() != ioutil.Discard {
		b, err := json.Marshal(o)
		if err != nil {
			o.logger.Printf("initialized processor '%s': %+v", processorType, o)
			return nil
		}
		o.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (o *OverrideTS) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		now := time.Now()
		o.logger.Printf("setting timestamp to %d with precision %s", now.UnixNano(), o.Precision)
		switch o.Precision {
		case "s":
			e.Timestamp = now.Unix()
		case "ms":
			e.Timestamp = now.UnixNano() / 1000000
		case "us":
			e.Timestamp = now.UnixNano() / 1000
		case "ns":
			e.Timestamp = now.UnixNano()
		}
	}
	return es
}
