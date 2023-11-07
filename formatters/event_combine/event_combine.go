// © 2023 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_combine_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/openconfig/gnmic/formatters"
	"github.com/openconfig/gnmic/types"
	"github.com/openconfig/gnmic/utils"
)

const (
	processorType = "event-combine"
	loggingPrefix = "[" + processorType + "] "
)

// combine allows running multiple processors together based on conditions
type combine struct {
	Processors []*procseq `mapstructure:"processors,omitempty"`
	Debug      bool       `mapstructure:"debug,omitempty"`

	processorsDefinitions map[string]map[string]any
	targetsConfigs        map[string]*types.TargetConfig
	actionsDefinitions    map[string]map[string]any

	logger *log.Logger
}

type procseq struct {
	Condition string `mapstructure:"condition,omitempty"`
	Name      string `mapstructure:"name,omitempty"`

	condition *gojq.Code
	proc      formatters.EventProcessor
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &combine{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *combine) Init(cfg any, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}
	if len(p.Processors) == 0 {
		return fmt.Errorf("missing processors definition")
	}
	for i, proc := range p.Processors {
		if proc == nil {
			return fmt.Errorf("missing processor(#%d) definition", i)
		}
		if proc.Name == "" {
			return fmt.Errorf("invalid processor(#%d) definition: missing name", i)
		}
		// init condition if it's set
		if proc.Condition != "" {
			proc.Condition = strings.TrimSpace(proc.Condition)
			q, err := gojq.Parse(proc.Condition)
			if err != nil {
				return err
			}
			proc.condition, err = gojq.Compile(q)
			if err != nil {
				return err
			}
		}
		// init subprocessors
		if epCfg, ok := p.processorsDefinitions[proc.Name]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				proc.proc = in()
				err := proc.proc.Init(epCfg[epType],
					formatters.WithLogger(p.logger),
					formatters.WithTargets(p.targetsConfigs),
					formatters.WithActions(p.actionsDefinitions),
					formatters.WithProcessors(p.actionsDefinitions),
				)
				if err != nil {
					return fmt.Errorf("failed initializing event processor '%s' of type='%s': %v", proc.Name, epType, err)
				}
				p.logger.Printf("added event processor '%s' of type=%s to combine processor", proc.Name, epType)
				continue
			}
			return fmt.Errorf("%q event processor has an unknown type=%q", proc.Name, epType)
		}
		return fmt.Errorf("%q event processor not found", proc.Name)
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

func (p *combine) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	les := len(es)

	in := make([]*formatters.EventMsg, 0, les)
	out := make([]*formatters.EventMsg, 0, les)
	for _, proc := range p.Processors {
		in = in[:0]
		out = out[:0]

		for i, e := range es {
			ok, err := formatters.CheckCondition(proc.condition, e)
			if err != nil {
				p.logger.Printf("condition check failed: %v", err)
			}
			if ok {
				if p.Debug {
					p.logger.Printf("processor #%d include: %s", i, e)
				}
				in = append(in, e)
				continue
			}
			if p.Debug {
				p.logger.Printf("processor #%d exclude: %s", i, e)
			}
			out = append(out, e)
		}

		in = proc.proc.Apply(in...)
		es = es[:0]
		es = append(es, in...)
		es = append(es, out...)
		if len(es) > 1 {
			sort.Slice(es, func(i, j int) bool {
				return es[i].Timestamp < es[j].Timestamp
			})
		}
	}
	return es
}

func (s *combine) WithLogger(l *log.Logger) {
	if s.Debug && l != nil {
		s.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if s.Debug {
		s.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (s *combine) WithTargets(tcs map[string]*types.TargetConfig) {
	s.targetsConfigs = tcs
}

func (s *combine) WithActions(act map[string]map[string]any) {
	s.actionsDefinitions = act
}

func (s *combine) WithProcessors(procs map[string]map[string]any) {
	s.processorsDefinitions = procs
}
