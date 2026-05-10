// © 2023 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_combine

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	processorType = "event-combine"
)

// combine allows running multiple processors together based on conditions
type combine struct {
	formatters.BaseProcessor
	Processors []*procseq `mapstructure:"processors,omitempty"`
	Debug      bool       `mapstructure:"debug,omitempty"`

	processorsDefinitions map[string]map[string]any
	targetsConfigs        map[string]*types.TargetConfig
	actionsDefinitions    map[string]map[string]any
}

type procseq struct {
	Condition string `mapstructure:"condition,omitempty"`
	Name      string `mapstructure:"name,omitempty"`

	condition *gojq.Code
	proc      formatters.EventProcessor
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &combine{}
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
	if p.Logger == nil {
		p.Logger = logging.DiscardLogger()
	}
	p.Logger = p.Logger.With("processor", processorType)
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
					formatters.WithLogger(p.Logger),
					formatters.WithTargets(p.targetsConfigs),
					formatters.WithActions(p.actionsDefinitions),
					formatters.WithProcessors(p.processorsDefinitions),
				)
				if err != nil {
					return fmt.Errorf("failed initializing event processor '%s' of type='%s': %v", proc.Name, epType, err)
				}
				p.Logger.Info("added event processor to combine processor", "name", proc.Name, "type", epType)
				continue
			}
			return fmt.Errorf("%q event processor has an unknown type=%q", proc.Name, epType)
		}
		return fmt.Errorf("%q event processor not found", proc.Name)
	}
	if p.Debug {
		if b, err := json.Marshal(p); err == nil {
			p.Logger.Debug("initialized processor", "config", string(b))
		} else {
			p.Logger.Debug("initialized processor", "config", p)
		}
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
				p.Logger.Warn("condition check failed", "err", err)
			}
			if ok {
				p.Logger.Debug("processor include", "index", i, "event", e)
				in = append(in, e)
				continue
			}
			p.Logger.Debug("processor exclude", "index", i, "event", e)
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

func (s *combine) WithLogger(l *slog.Logger) {
	if !s.Debug {
		l = nil
	}
	s.BaseProcessor.WithLogger(l)
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
