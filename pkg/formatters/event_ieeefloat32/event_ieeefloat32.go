// © 2024 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_ieeefloat32

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"regexp"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
)

const (
	processorType = "event-ieeefloat32"
	loggingPrefix = "[" + processorType + "] "
)

// ieeefloat32 converts values from a base64 encoded string into a float32
type ieeefloat32 struct {
	Condition  string   `mapstructure:"condition,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	valueNames []*regexp.Regexp
	code       *gojq.Code
	logger     *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &ieeefloat32{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *ieeefloat32) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.Condition != "" {
		p.Condition = strings.TrimSpace(p.Condition)
		q, err := gojq.Parse(p.Condition)
		if err != nil {
			return err
		}
		p.code, err = gojq.Compile(q)
		if err != nil {
			return err
		}
	}

	// init value names regex
	p.valueNames, err = compileRegex(p.ValueNames)
	if err != nil {
		return err
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

func (p *ieeefloat32) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		// condition is set
		if p.code != nil && p.Condition != "" {
			ok, err := formatters.CheckCondition(p.code, e)
			if err != nil {
				p.logger.Printf("condition check failed: %v", err)
			}
			if !ok {
				continue
			}
		}
		// condition passed => check regexes
		for k, v := range e.Values {
			for _, re := range p.valueNames {
				if re.MatchString(k) {
					f, err := p.decodeBase64String(v)
					if err != nil {
						p.logger.Printf("failed to decode base64 string: %v", err)
						continue
					}
					e.Values[k] = f
					break
				}
			}
		}
	}
	return es
}

func (p *ieeefloat32) WithLogger(l *log.Logger) {
	if p.Debug && l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *ieeefloat32) WithTargets(tcs map[string]*types.TargetConfig) {}

func (p *ieeefloat32) WithActions(act map[string]map[string]interface{}) {}

func (p *ieeefloat32) WithProcessors(procs map[string]map[string]any) {}

func (p *ieeefloat32) decodeBase64String(e any) (float32, error) {
	switch b64 := e.(type) {
	default:
		return 0, fmt.Errorf("invalid type: %T", e)
	case string:

		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return 0, fmt.Errorf("failed to decode base64: %v", err)
		}

		if len(data) < 4 {
			return 0, fmt.Errorf("decoded data is less than 4 bytes")
		}
		bits := binary.BigEndian.Uint32(data[:4])
		floatVal := math.Float32frombits(bits)
		return floatVal, nil
	}
}

func compileRegex(expr []string) ([]*regexp.Regexp, error) {
	res := make([]*regexp.Regexp, 0, len(expr))
	for _, reg := range expr {
		re, err := regexp.Compile(reg)
		if err != nil {
			return nil, err
		}
		res = append(res, re)
	}
	return res, nil
}
