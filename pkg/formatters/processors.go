// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package formatters

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/itchyny/gojq"
	"github.com/mitchellh/mapstructure"

	"github.com/openconfig/gnmic/pkg/types"
)

var EventProcessors = map[string]Initializer{}

var EventProcessorTypes = []string{
	"event-add-tag",
	"event-allow",
	"event-convert",
	"event-date-string",
	"event-delete",
	"event-drop",
	"event-extract-tags",
	"event-jq",
	"event-merge",
	"event-override-ts",
	"event-rate-limit",
	"event-strings",
	"event-to-tag",
	"event-trigger",
	"event-write",
	"event-group-by",
	"event-data-convert",
	"event-value-tag",
	"event-starlark",
	"event-combine",
}

type Initializer func() EventProcessor

func Register(name string, initFn Initializer) {
	EventProcessors[name] = initFn
}

type Option func(EventProcessor)
type EventProcessor interface {
	Init(interface{}, ...Option) error
	Apply(...*EventMsg) []*EventMsg

	WithTargets(map[string]*types.TargetConfig)
	WithLogger(l *log.Logger)
	WithActions(act map[string]map[string]interface{})
	WithProcessors(procs map[string]map[string]any)
}

func DecodeConfig(src, dst interface{}) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     dst,
		},
	)
	if err != nil {
		return err
	}
	return decoder.Decode(src)
}

func WithLogger(l *log.Logger) Option {
	return func(p EventProcessor) {
		p.WithLogger(l)
	}
}

func WithTargets(tcs map[string]*types.TargetConfig) Option {
	return func(p EventProcessor) {
		p.WithTargets(tcs)
	}
}

func WithActions(acts map[string]map[string]interface{}) Option {
	return func(p EventProcessor) {
		p.WithActions(acts)
	}
}

func WithProcessors(procs map[string]map[string]interface{}) Option {
	return func(p EventProcessor) {
		p.WithProcessors(procs)
	}
}

func CheckCondition(code *gojq.Code, e *EventMsg) (bool, error) {
	if code == nil {
		return true, nil
	}

	var res interface{}

	input := make(map[string]interface{})
	b, err := json.Marshal(e)
	if err != nil {
		return false, err
	}
	err = json.Unmarshal(b, &input)
	if err != nil {
		return false, err
	}
	iter := code.Run(input)
	if err != nil {
		return false, err
	}
	var ok bool
	res, ok = iter.Next()
	// iterator not done, so the final result won't be a boolean
	if !ok {
		//
		return false, nil
	}
	if err, ok = res.(error); ok {
		return false, err
	}

	switch res := res.(type) {
	case bool:
		return res, nil
	default:
		return false, fmt.Errorf("unexpected condition return type: %T | %v", res, res)
	}
}

func MakeEventProcessors(
	logger *log.Logger,
	processorNames []string,
	ps map[string]map[string]interface{},
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{},
) ([]EventProcessor, error) {
	evps := make([]EventProcessor, len(processorNames))
	for i, epName := range processorNames {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType],
					WithLogger(logger),
					WithTargets(tcs),
					WithActions(acts),
					WithProcessors(ps),
				)
				if err != nil {
					return nil, fmt.Errorf("failed initializing event processor '%s' of type='%s': %w", epName, epType, err)
				}
				evps[i] = ep
				logger.Printf("added event processor '%s' of type=%s to output", epName, epType)
				continue
			}
			return nil, fmt.Errorf("%q event processor has an unknown type=%q", epName, epType)
		}
		return nil, fmt.Errorf("%q event processor not found", epName)
	}
	return evps, nil
}
