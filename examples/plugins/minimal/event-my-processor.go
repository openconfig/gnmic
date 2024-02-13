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
	// TODO: Choose a name for your processor
	processorType = "event-my-processor"
)

type myProcessor struct {
	// TODO: Add your config struct fields here
}

func (p *myProcessor) Init(cfg interface{}, opts ...formatters.Option) error {
	// decode the plugin config
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	// apply options
	for _, o := range opts {
		o(p)
	}
	// TODO: Other initialization steps...
	return nil
}

func (p *myProcessor) Apply(event ...*formatters.EventMsg) []*formatters.EventMsg {
	// TODO: The processor's logic is applied here
	return event
}

func (p *myProcessor) WithActions(act map[string]map[string]interface{}) {
}

func (p *myProcessor) WithTargets(tcs map[string]*types.TargetConfig) {
}

func (p *myProcessor) WithProcessors(procs map[string]map[string]any) {
}

func (p *myProcessor) WithLogger(l *log.Logger) {
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Output:      os.Stderr,
		DisableTime: true,
	})

	logger.Info("starting plugin processor", "name", processorType)

	// TODO: Create and initialize your processor's struct
	plug := &myProcessor{}
	// start it
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
