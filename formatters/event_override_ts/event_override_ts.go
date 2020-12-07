package event_override_ts

import (
	"io/ioutil"
	"log"
	"time"

	"github.com/karimra/gnmic/formatters"
)

// OverrideTS Drops the msg if ANY of the Tags or Values regexes are matched
type OverrideTS struct {
	Precision string `mapstructure:"precision,omitempty"`
	Debug     bool   `mapstructure:"debug,omitempty"`

	logger *log.Logger
}

func init() {
	formatters.Register("event_override_ts", func() formatters.EventProcessor {
		return &OverrideTS{}
	})
}

func (o *OverrideTS) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, 0)
	if err != nil {
		return err
	}
	if o.Precision == "" {
		o.Precision = "ns"
	}
	if o.Debug {
		o.logger = log.New(logger.Writer(), "event_override_ts ", logger.Flags())
	} else {
		o.logger = log.New(ioutil.Discard, "", 0)
	}
	return nil
}

func (o *OverrideTS) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
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
