// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_starlark

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"go.starlark.net/lib/math"
	"go.starlark.net/lib/time"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/types"
	"github.com/openconfig/gnmic/pkg/utils"
)

const (
	processorType = "event-starlark"
	loggingPrefix = "[" + processorType + "] "
)

// starlarkProc runs a starlark script on the received events
type starlarkProc struct {
	Script string `mapstructure:"script,omitempty" json:"script,omitempty"`
	Source string `mapstructure:"source,omitempty" json:"source,omitempty"`
	Debug  bool   `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	// this mutex ensures batches of events are processed in sequence
	m       sync.Mutex
	thread  *starlark.Thread
	applyFn starlark.Value
	logger  *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &starlarkProc{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *starlarkProc) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}

	err = p.validate()
	if err != nil {
		return err
	}

	p.thread = &starlark.Thread{
		Print: func(_ *starlark.Thread, msg string) {
			p.logger.Printf("print(): %v", msg)
		},
		Load: func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
			return loadModule(module)
		},
	}
	// sourceProgram
	builtins := starlark.StringDict{}
	builtins["Event"] = starlark.NewBuiltin("Event", newEvent)
	builtins["copy_event"] = starlark.NewBuiltin("copy_event", copyEvent)
	prog, err := p.sourceProgram(builtins)
	if err != nil {
		return err
	}
	globals, err := prog.Init(p.thread, builtins)
	if err != nil {
		return err
	}
	if !globals.Has("apply") {
		return errors.New("missing global function apply")
	}
	p.applyFn = globals["apply"]

	globals["cache"] = starlark.NewDict(0)

	globals.Freeze()
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

func (p *starlarkProc) validate() error {
	if p.Source == "" && p.Script == "" {
		return errors.New("one of 'script' or 'source' must be set")
	}
	if p.Source != "" && p.Script != "" {
		return errors.New("only one of 'script' or 'source' can be set")
	}
	return nil
}

func (p *starlarkProc) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	p.m.Lock()
	defer p.m.Unlock()
	numMsgs := len(es)
	if numMsgs == 0 {
		return es
	}
	sevs := make([]starlark.Value, 0, numMsgs)
	for _, ev := range es {
		if ev.Tags == nil {
			ev.Tags = make(map[string]string)
		}
		if ev.Values == nil {
			ev.Values = make(map[string]any)
		}
		if ev.Deletes == nil {
			ev.Deletes = make([]string, 0)
		}
		sevs = append(sevs, fromEvent(ev))
	}
	if len(sevs) == 0 {
		return es
	}
	if p.Debug {
		p.logger.Printf("events input: %v", sevs)
	}
	r, err := starlark.Call(p.thread, p.applyFn, sevs, nil)
	if err != nil {
		if p.Debug {
			p.logger.Printf("failed to run script with input %v: %v", sevs, err)
		} else {
			p.logger.Printf("failed to run script: %v", err)
		}
		return es
	}
	if p.Debug {
		p.logger.Printf("script output: %+v", r)
	}
	// r must implement .Iterate() and .Len()
	if r, ok := r.(starlark.Sequence); ok {
		res := make([]*formatters.EventMsg, 0, r.Len())
		iter := r.Iterate()
		defer r.Iterate().Done()
		for {
			var v starlark.Value
			ok := iter.Next(&v)
			if !ok {
				break
			}
			switch v := v.(type) {
			case *event:
				res = append(res, toEvent(v))
			default:
				p.logger.Printf("unexpected return type: %T", v)
				continue
			}
		}
		if p.Debug {
			p.logger.Printf("resulting events: %v", res)
		}
		return res
	}
	p.logger.Printf("unexpected script output format, expecting a Sequence of Event, got %T", r)
	return es
}

func (p *starlarkProc) WithLogger(l *log.Logger) {
	if l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *starlarkProc) WithTargets(tcs map[string]*types.TargetConfig) {}

func (p *starlarkProc) WithActions(act map[string]map[string]interface{}) {}

func (p *starlarkProc) WithProcessors(procs map[string]map[string]any) {}

func (p *starlarkProc) sourceProgram(builtins starlark.StringDict) (*starlark.Program, error) {
	var src any
	if p.Source != "" {
		src = p.Source
	}
	options := &syntax.FileOptions{
		Set:            true,
		GlobalReassign: true,
		Recursion:      true,
	}
	_, program, err := starlark.SourceProgramOptions(options, p.Script, src, builtins.Has)
	return program, err
}

func loadModule(module string) (starlark.StringDict, error) {
	switch module {
	case "math.star":
		return starlark.StringDict{
			"math": math.Module,
		}, nil
	case "time.star":
		return starlark.StringDict{
			"time": time.Module,
		}, nil
	default:
		return nil, fmt.Errorf("module %q unknown", module)
	}
}
