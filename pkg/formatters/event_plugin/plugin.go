package event_plugin

import (
	"net/rpc"

	"github.com/hashicorp/go-plugin"

	"github.com/openconfig/gnmic/pkg/formatters"
)

type EventProcessorPlugin struct {
	Impl formatters.EventProcessor
}

func (p *EventProcessorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &eventProcessorRPCServer{Impl: p.Impl}, nil
}

func (p *EventProcessorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &EventProcessorRPC{client: c}, nil
}
