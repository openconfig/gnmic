// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_time_epoch

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
)

const (
	processorType = "event-time-epoch"
	loggingPrefix = "[" + processorType + "] "
)

// epoch converts a time string to epoch time
type epoch struct {
	Values    []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Precision string   `mapstructure:"precision,omitempty" json:"precision,omitempty"`
	Format    string   `mapstructure:"format,omitempty" json:"format,omitempty"`
	Debug     bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	values []*regexp.Regexp
	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &epoch{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (d *epoch) Init(cfg any, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(d)
	}
	if d.Format == "" {
		d.Format = time.RFC3339
	}
	// init values regex
	d.values = make([]*regexp.Regexp, 0, len(d.Values))
	for _, reg := range d.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.values = append(d.values, re)
	}
	if d.logger.Writer() != io.Discard {
		b, err := json.Marshal(d)
		if err != nil {
			d.logger.Printf("initialized processor '%s': %+v", processorType, d)
			return nil
		}
		d.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (d *epoch) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		for k, v := range e.Values {
			for _, re := range d.values {
				if re.MatchString(k) {
					d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
					switch v := v.(type) {
					case string:
						td, err := time.Parse(d.Format, v)
						if err != nil {
							d.logger.Printf("failed to convert '%v' to time: %v", v, err)
							continue
						}
						var ts int64
						switch d.Precision {
						case "s", "sec", "second":
							ts = td.Unix()
						case "ms", "millisecond":
							ts = td.UnixMilli()
						case "us", "microsecond":
							ts = td.UnixMicro()
						case "ns", "nanosecond":
							ts = td.UnixNano()
						default:
							ts = td.UnixNano()
						}
						e.Values[k] = ts
					default:
					}
					break
				}
			}
		}
	}
	return es
}

func (d *epoch) WithLogger(l *log.Logger) {
	if d.Debug && l != nil {
		d.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if d.Debug {
		d.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (d *epoch) WithTargets(tcs map[string]*types.TargetConfig) {}

func (d *epoch) WithActions(act map[string]map[string]any) {}

func (d *epoch) WithProcessors(procs map[string]map[string]any) {}
