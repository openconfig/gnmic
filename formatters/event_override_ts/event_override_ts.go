package event_override_ts

import (
	"time"

	"github.com/karimra/gnmic/formatters"
)

// OverrideTS Drops the msg if ANY of the Tags or Values regexes are matched
type OverrideTS struct {
	Unit string `mapstructure:"unit,omitempty"`
}

func init() {
	formatters.Register("event_override_ts", func() formatters.EventProcessor {
		return &OverrideTS{}
	})
}

func (o *OverrideTS) Init(cfg interface{}) error {
	err := formatters.DecodeConfig(cfg, 0)
	if err != nil {
		return err
	}
	if o.Unit == "" {
		o.Unit = "ms"
	}
	return nil
}

func (o *OverrideTS) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	now := time.Now()
	switch o.Unit {
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
