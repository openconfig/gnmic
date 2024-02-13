package main

import (
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/formatters/event_plugin"
)

const (
	processorType = "event-go-plugin"
	loggingPrefix = "[" + processorType + "] "
)

type goSampleProcessorPlugin struct {
	Debug bool `mapstructure:"debug,omitempty" yaml:"debug,omitempty" json:"debug,omitempty"`

	targetsConfigs        map[string]*types.TargetConfig
	actionsDefinitions    map[string]map[string]interface{}
	processorsDefinitions map[string]map[string]any
	logger                hclog.Logger
}

func (p *goSampleProcessorPlugin) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, o := range opts {
		o(p)
	}
	// initialize logger
	p.logger.Info("initializing", "processor", processorType, "cfg", cfg)
	// initialize your processor's config and handle the options
	// - set default
	// - validate config
	p.logger.Info("initialized", "processor", processorType)
	return nil
}

func (p *goSampleProcessorPlugin) Apply(event ...*formatters.EventMsg) []*formatters.EventMsg {
	// apply the processor's logic here
	// return the new/modified event messages
	return event
}

func (p *goSampleProcessorPlugin) WithActions(act map[string]map[string]interface{}) {
	p.actionsDefinitions = act
}

func (p *goSampleProcessorPlugin) WithTargets(tcs map[string]*types.TargetConfig) {
	p.targetsConfigs = tcs
}

func (p *goSampleProcessorPlugin) WithProcessors(procs map[string]map[string]any) {
	p.processorsDefinitions = procs
}

func (p *goSampleProcessorPlugin) WithLogger(l *log.Logger) {
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Output:      os.Stderr,
		DisableTime: true,
	})

	logger.Info("starting plugin processor", "name", processorType)

	plug := &goSampleProcessorPlugin{}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "GNMIC_PLUGIN",
			MagicCookieValue: "gnmic",
		},
		Plugins: map[string]plugin.Plugin{
			processorType: &event_plugin.EventProcessorPlugin{Impl: plug},
		},
		Logger: logger,
	})
}
