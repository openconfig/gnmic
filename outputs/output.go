package outputs

import (
	"context"
	"log"
	"net"
	"strings"
	"text/template"

	"github.com/karimra/gnmic/formatters"
	_ "github.com/karimra/gnmic/formatters/all"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

type Output interface {
	Init(context.Context, string, map[string]interface{}, ...Option) error
	Write(context.Context, proto.Message, Meta)
	WriteEvent(context.Context, *formatters.EventMsg)
	Close() error
	RegisterMetrics(*prometheus.Registry)
	String() string

	SetLogger(*log.Logger)
	SetEventProcessors(map[string]map[string]interface{}, *log.Logger, map[string]interface{})
	SetName(string)
	SetClusterName(string)
}

type Initializer func() Output

var Outputs = map[string]Initializer{}

var OutputTypes = []string{
	"file",
	"influxdb",
	"kafka",
	"nats",
	"prometheus",
	"stan",
	"tcp",
	"udp",
	"gnmi",
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

func AddSubscriptionTarget(msg proto.Message, meta Meta, addTarget string, tpl *template.Template) error {
	if addTarget == "" {
		return nil
	}
	switch trsp := msg.(type) {
	case *gnmi.SubscribeResponse:
		switch trsp := trsp.Response.(type) {
		case *gnmi.SubscribeResponse_Update:
			if trsp.Update.Prefix == nil {
				trsp.Update.Prefix = new(gnmi.Path)
			}
			switch addTarget {
			case "overwrite":
				sb := new(strings.Builder)
				err := tpl.Execute(sb, meta)
				if err != nil {
					return err
				}
				trsp.Update.Prefix.Target = sb.String()
			case "if-not-present":
				if trsp.Update.Prefix.Target == "" {
					sb := new(strings.Builder)
					err := tpl.Execute(sb, meta)
					if err != nil {
						return err
					}
					trsp.Update.Prefix.Target = sb.String()
				}
			}
		}
	}
	return nil
}

var (
	DefaultTargetTemplate = template.Must(
		template.New("target-template").
			Funcs(TemplateFuncs).
			Parse(defaultTargetTemplateString))
)

var TemplateFuncs = template.FuncMap{
	"host": GetHost,
}

const (
	defaultTargetTemplateString = `
{{- if index . "subscription-target" -}}
{{ index . "subscription-target" }}
{{- else -}}
{{ index . "source" | host }}
{{- end -}}`
)

func GetHost(hostport string) string {
	h, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return h
}
