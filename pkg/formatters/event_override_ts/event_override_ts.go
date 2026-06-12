// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_override_ts

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	processorType = "event-override-ts"
)

// overrideTS Overrides the message timestamp with the local time
type overrideTS struct {
	formatters.BaseProcessor

	Precision string `mapstructure:"precision,omitempty" json:"precision,omitempty"`
	Debug     bool   `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &overrideTS{}
	})
}

func (o *overrideTS) Init(cfg any, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, o)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.Logger == nil {
		o.Logger = logging.DiscardLogger()
	}
	o.Logger = o.Logger.With("processor", processorType)
	if o.Precision == "" {
		o.Precision = "ns"
	}
	if o.Logger.Enabled(context.Background(), slog.LevelDebug) {
		if b, err := json.Marshal(o); err == nil {
			o.Logger.Debug("initialized processor", "config", string(b))
		} else {
			o.Logger.Debug("initialized processor", "config", o)
		}
	}
	return nil
}

func (o *overrideTS) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		now := time.Now()
		o.Logger.Debug("setting timestamp", "timestamp", now.UnixNano(), "precision", o.Precision)
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

func (o *overrideTS) WithLogger(l *slog.Logger) {
	if !o.Debug {
		l = nil
	}
	o.BaseProcessor.WithLogger(l)
}
