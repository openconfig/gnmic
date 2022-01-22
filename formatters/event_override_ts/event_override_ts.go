package event_override_ts

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

const (
	processorType = "event-override-ts"
	loggingPrefix = "[" + processorType + "] "
)

// OverrideTS Overrides the message timestamp with the local time
type OverrideTS struct {
	//formatters.EventProcessor

	Precision string `mapstructure:"precision,omitempty" json:"precision,omitempty"`
	Debug     bool   `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &OverrideTS{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (o *OverrideTS) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, o)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.Precision == "" {
		o.Precision = "ns"
	}
	if o.logger.Writer() != io.Discard {
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

func (o *OverrideTS) WithLogger(l *log.Logger) {
	if o.Debug && l != nil {
		o.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if o.Debug {
		o.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (o *OverrideTS) WithTargets(tcs map[string]*types.TargetConfig) {}

func (o *OverrideTS) WithActions(act map[string]map[string]interface{}) {}
