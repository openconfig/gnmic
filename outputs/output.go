// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package outputs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/openconfig/gnmic/formatters"
	_ "github.com/openconfig/gnmic/formatters/all"
	"github.com/openconfig/gnmic/types"
	"github.com/openconfig/gnmic/utils"
)

type Output interface {
	Init(context.Context, string, map[string]interface{}, ...Option) error
	Write(context.Context, proto.Message, Meta)
	WriteEvent(context.Context, *formatters.EventMsg)
	Close() error
	RegisterMetrics(*prometheus.Registry)
	String() string

	SetLogger(*log.Logger)
	SetEventProcessors(map[string]map[string]interface{}, *log.Logger, map[string]*types.TargetConfig, map[string]map[string]interface{})
	SetName(string)
	SetClusterName(string)
	SetTargetsConfig(map[string]*types.TargetConfig)
}

type Initializer func() Output

var Outputs = map[string]Initializer{}

var OutputTypes = map[string]struct{}{
	"file":             {},
	"influxdb":         {},
	"kafka":            {},
	"nats":             {},
	"prometheus":       {},
	"prometheus_write": {},
	"stan":             {},
	"tcp":              {},
	"udp":              {},
	"gnmi":             {},
	"jetstream":        {},
	"snmp":             {},
	"asciigraph":       {},
}

func Register(name string, initFn Initializer) {
	Outputs[name] = initFn
}

type Meta map[string]string

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

func AddSubscriptionTarget(msg proto.Message, meta Meta, addTarget string, tpl *template.Template) (*gnmi.SubscribeResponse, error) {
	if addTarget == "" {
		if message, ok := msg.(*gnmi.SubscribeResponse); ok {
			return message, nil
		}
		return nil, nil
	}
	msg = proto.Clone(msg)
	switch trsp := msg.(type) {
	case *gnmi.SubscribeResponse:
		switch rrsp := trsp.Response.(type) {
		case *gnmi.SubscribeResponse_Update:
			if rrsp.Update.Prefix == nil {
				rrsp.Update.Prefix = new(gnmi.Path)
			}
			switch addTarget {
			case "overwrite":
				sb := new(strings.Builder)
				err := tpl.Execute(sb, meta)
				if err != nil {
					return nil, err
				}
				rrsp.Update.Prefix.Target = sb.String()
				return trsp, nil
			case "if-not-present":
				if rrsp.Update.Prefix.Target == "" {
					sb := new(strings.Builder)
					err := tpl.Execute(sb, meta)
					if err != nil {
						return nil, err
					}
					rrsp.Update.Prefix.Target = sb.String()
				}
				return trsp, nil
			}
		}
	}
	return nil, nil
}

func ExecTemplate(content []byte, tpl *template.Template) ([]byte, error) {
	var input interface{}
	err := json.Unmarshal(content, &input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %v", err)
	}
	bf := new(bytes.Buffer)
	err = tpl.Execute(bf, input)
	if err != nil {
		return nil, fmt.Errorf("failed to execute msg template: %v", err)
	}
	return bf.Bytes(), nil
}

var (
	DefaultTargetTemplate = template.Must(
		template.New("target-template").
			Funcs(TemplateFuncs).
			Parse(defaultTargetTemplateString))
)

var TemplateFuncs = template.FuncMap{
	"host": utils.GetHost,
}

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
