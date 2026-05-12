package main

import (
	"log/slog"
	"os"

	"github.com/hashicorp/go-plugin"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/formatters/event_plugin"
)

const (
	processorType = "event-add-device_function"
)

type MyEventProcessor struct {
	formatters.BaseProcessor
	Debug bool `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	targetsConfigs        map[string]*types.TargetConfig
	actionsDefinitions    map[string]map[string]interface{}
	processorsDefinitions map[string]map[string]any
	logger                *slog.Logger
}

func (p *MyEventProcessor) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	p.setupLogger()

	if err != nil {
		return err
	}

	return nil
}

func (p *MyEventProcessor) Apply(event ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range event {
		if e.Tags == nil {
			e.Tags = make(map[string]string)
		}
		e.Tags["device_function"] = "CORE"

	}
	return event
}

func (p *MyEventProcessor) Close() error {
	return nil
}

func (p *MyEventProcessor) WithActions(act map[string]map[string]interface{}) {
	p.actionsDefinitions = act
}

func (p *MyEventProcessor) WithTargets(tcs map[string]*types.TargetConfig) {
	p.targetsConfigs = tcs
}

func (p *MyEventProcessor) WithProcessors(procs map[string]map[string]any) {
	p.processorsDefinitions = procs
}

func (p *MyEventProcessor) WithLogger(l *slog.Logger) {
	p.logger = l
}

func (p *MyEventProcessor) setupLogger() {
	if !p.Debug {
		p.logger = slog.New(slog.DiscardHandler)
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	logger.Info("starting plugin")
	plug := &MyEventProcessor{logger: logger}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "GNMIC_PLUGIN",
			MagicCookieValue: "gnmic",
		},
		Plugins: map[string]plugin.Plugin{
			processorType: &event_plugin.EventProcessorPlugin{Impl: plug},
		},
		Logger: nil,
	})
}
