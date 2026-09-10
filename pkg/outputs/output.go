// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

// © 2025 NVIDIA Corporation
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of NVIDIA's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package outputs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"text/template"

	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	_ "github.com/openconfig/gnmic/pkg/formatters/all"
	"github.com/openconfig/gnmic/pkg/logging"
	pkgutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
)

// BindLogger returns a non-nil *slog.Logger annotated with the output kind
// and instance name. Centralises the
//
//	if options.Logger != nil { o.logger = options.Logger.With(...) }
//
// boilerplate that every output used to repeat. When parent is nil, a
// discarding logger is returned so callers never need a nil check.
func BindLogger(parent *slog.Logger, outputType, name string) *slog.Logger {
	return logging.Component(parent, "output", outputType, name)
}

type Output interface {
	// initialize the output
	Init(context.Context, string, map[string]any, ...Option) error
	// validate the config
	Validate(map[string]any) error
	// update the config
	Update(context.Context, map[string]any) error
	// update a processor
	UpdateProcessor(string, map[string]any) error
	// write a protobuf message to the output
	Write(context.Context, proto.Message, Meta)
	// write an event message to the output
	WriteEvent(context.Context, *formatters.EventMsg)
	// close the output
	Close() error
	// return a string representation of the output
	String() string
}

type Initializer func() Output

var Outputs = map[string]Initializer{}

var OutputTypes = map[string]struct{}{
	"asciigraph":       {},
	"clickhouse":       {},
	"file":             {},
	"influxdb":         {},
	"kafka":            {},
	"nats":             {},
	"otlp":             {},
	"prometheus":       {},
	"prometheus_write": {},
	"tcp":              {},
	"udp":              {},
	"gnmi":             {},
	"jetstream":        {},
	"snmp":             {},
}

func Register(name string, initFn Initializer) {
	Outputs[name] = initFn
}

var bytesBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var stringBuilderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

type Meta map[string]string

// DecodeConfig decodes an output configuration map into dst.
//
// On top of the mapstructure decoding, it validates the fields shared by
// several output types (currently `add-target`), so that a typo in one of
// them is rejected at load time instead of silently changing the output
// behavior at runtime.
func DecodeConfig(src, dst any) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     dst,
		},
	)
	if err != nil {
		return err
	}
	err = decoder.Decode(src)
	if err != nil {
		return err
	}
	if m, ok := src.(map[string]any); ok {
		if v, ok := m["add-target"]; ok {
			return ValidateAddTarget(v)
		}
	}
	return nil
}

const (
	// AddTargetOverwrite always sets Prefix.Target from the target template.
	AddTargetOverwrite = "overwrite"
	// AddTargetIfNotPresent sets Prefix.Target from the target template only
	// when the received message has an empty target.
	AddTargetIfNotPresent = "if-not-present"
)

// ValidateAddTarget checks the value of an output `add-target` field.
// An empty string (or nil) disables the feature and is valid.
func ValidateAddTarget(v any) error {
	switch v := v.(type) {
	case nil:
		return nil
	case string:
		switch v {
		case "", AddTargetOverwrite, AddTargetIfNotPresent:
			return nil
		}
		return fmt.Errorf("invalid add-target value %q: expected %q or %q", v, AddTargetOverwrite, AddTargetIfNotPresent)
	default:
		return fmt.Errorf("invalid add-target value %v of type %T: expected %q or %q", v, v, AddTargetOverwrite, AddTargetIfNotPresent)
	}
}

// AddSubscriptionTarget is the proto.Message form of AddSubscribeResponseTarget.
// Messages that are not a *gnmi.SubscribeResponse are returned as they were
// received: the function never returns a nil message for a non-nil input.
func AddSubscriptionTarget(msg proto.Message, meta Meta, addTarget string, tpl *template.Template) (proto.Message, error) {
	rsp, ok := msg.(*gnmi.SubscribeResponse)
	if !ok || rsp == nil {
		return msg, nil
	}
	return AddSubscribeResponseTarget(rsp, meta, addTarget, tpl)
}

// AddSubscribeResponseTarget sets the Prefix.Target of a SubscribeResponse update
// according to addTarget (see AddTargetOverwrite and AddTargetIfNotPresent),
// using tpl rendered with meta as the target value.
//
// Responses that need no change (addTarget empty or unknown, sync-responses,
// updates that already carry a target with AddTargetIfNotPresent) are returned
// as they were received: the function never returns nil for a non-nil input.
// When the response is modified, the input is left untouched and a modified
// copy is returned. On template error the original response is returned along
// with the error.
func AddSubscribeResponseTarget(rsp *gnmi.SubscribeResponse, meta Meta, addTarget string, tpl *template.Template) (*gnmi.SubscribeResponse, error) {
	if rsp == nil || addTarget == "" {
		return rsp, nil
	}
	upd := rsp.GetUpdate()
	if upd == nil {
		// sync-response or error: nothing to add a target to
		return rsp, nil
	}
	switch addTarget {
	case AddTargetOverwrite:
	case AddTargetIfNotPresent:
		if upd.GetPrefix().GetTarget() != "" {
			return rsp, nil
		}
	default:
		// unknown value, rejected by ValidateAddTarget at config time
		return rsp, nil
	}
	if tpl == nil {
		return rsp, fmt.Errorf("add-target is set to %q but no target template is configured", addTarget)
	}
	sb := stringBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringBuilderPool.Put(sb)
	}()
	err := tpl.Execute(sb, meta)
	if err != nil {
		return rsp, err
	}
	rsp = proto.Clone(rsp).(*gnmi.SubscribeResponse)
	upd = rsp.GetUpdate()
	if upd.Prefix == nil {
		upd.Prefix = new(gnmi.Path)
	}
	upd.Prefix.Target = sb.String()
	return rsp, nil
}

func ExecTemplate(content []byte, tpl *template.Template) ([]byte, error) {
	var input interface{}
	err := json.Unmarshal(content, &input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %v", err)
	}
	bf := bytesBufferPool.Get().(*bytes.Buffer)
	defer func() {
		bf.Reset()
		bytesBufferPool.Put(bf)
	}()
	err = tpl.Execute(bf, input)
	if err != nil {
		return nil, fmt.Errorf("failed to execute msg template: %v", err)
	}
	result := bf.Bytes()
	out := make([]byte, len(result))
	copy(out, result)
	return out, nil
}

var (
	DefaultTargetTemplate = template.Must(
		template.New("target-template").
			Funcs(TemplateFuncs).
			Parse(defaultTargetTemplateString))

	TemplateFuncs = template.FuncMap{
		"host": utils.GetHost,
	}
)

const (
	defaultTargetTemplateString = `
{{- if index . "subscription-target" -}}
{{ index . "subscription-target" }}
{{- else -}}
{{ index . "source" | host }}
{{- end -}}`
)

func Marshal(pmsg protoreflect.ProtoMessage, meta map[string]string, mo *formatters.MarshalOptions, splitEvents bool, evps ...formatters.EventProcessor) ([][]byte, error) {
	switch mo.Format {
	case "event":
		if splitEvents {
			return marshalSplit(pmsg, meta, mo, evps...)
		}
		fallthrough
	default:
		b, err := mo.Marshal(pmsg, meta, evps...)
		if err != nil {
			return nil, err
		}
		if len(b) == 0 {
			return nil, nil
		}
		return [][]byte{b}, nil
	}
}

func marshalSplit(pmsg protoreflect.ProtoMessage, meta map[string]string, mo *formatters.MarshalOptions, evps ...formatters.EventProcessor) ([][]byte, error) {
	var subscriptionName string
	var ok bool
	if subscriptionName, ok = meta["subscription-name"]; !ok {
		subscriptionName = "default"
	}
	switch msg := pmsg.(type) {
	case *gnmi.SubscribeResponse:
		switch msg.GetResponse().(type) {
		case *gnmi.SubscribeResponse_Update:
			events, err := formatters.ResponseToEventMsgs(subscriptionName, msg, meta, evps...)
			if err != nil {
				return nil, fmt.Errorf("failed converting response to events: %v", err)
			}
			numEvents := len(events)
			if numEvents == 0 {
				return nil, nil
			}
			rs := make([][]byte, 0, numEvents)
			marshalFn := json.Marshal
			if mo.Multiline {
				marshalFn = func(v any) ([]byte, error) {
					return json.MarshalIndent(v, "", mo.Indent)
				}
			}
			for _, ev := range events {
				b, err := marshalFn(ev)
				if err != nil {
					return nil, err
				}
				rs = append(rs, b)
			}
			return rs, nil
		default:
			return nil, fmt.Errorf("unexpected message type: %T", msg)
		}
	default:
		return nil, fmt.Errorf("unexpected message type: %T", msg)
	}
}

type BaseOutput struct {
}

func (b *BaseOutput) Init(context.Context, string, map[string]any, ...Option) error {
	return nil
}

func (b *BaseOutput) Validate(map[string]any) error {
	return nil
}

func (b *BaseOutput) Update(context.Context, map[string]any) error {
	return nil
}

func (b *BaseOutput) UpdateProcessor(string, map[string]any) error {
	return nil
}

func (b *BaseOutput) Write(context.Context, proto.Message, Meta) {}

func (b *BaseOutput) WriteEvent(context.Context, *formatters.EventMsg) {}

func (b *BaseOutput) Close() error {
	return nil
}

func (b *BaseOutput) String() string {
	return ""
}

// update processor helper

func UpdateProcessorInSlice(
	logger *slog.Logger,
	storeObj store.Store[any],
	eventProcessors []string,
	currentEvps []formatters.EventProcessor,
	processorName string,
	pcfg map[string]any,
) ([]formatters.EventProcessor, bool, error) {
	tcs, ps, acts, err := pkgutils.GetConfigMaps(storeObj)
	if err != nil {
		return nil, false, err
	}

	for i, epName := range eventProcessors {
		if epName == processorName {
			ep, err := formatters.MakeProcessor(logger, processorName, pcfg, ps, tcs, acts)
			if err != nil {
				return nil, false, err
			}

			if i >= len(currentEvps) {
				return nil, false, fmt.Errorf("output processors are not properly initialized")
			}

			// create new slice with updated processor
			newEvps := make([]formatters.EventProcessor, len(currentEvps))
			copy(newEvps, currentEvps)
			newEvps[i] = ep

			logger.Info("updated event processor", "processor", processorName)
			return newEvps, true, nil
		}
	}

	// processor not found - return currentEvps
	return currentEvps, false, nil
}
