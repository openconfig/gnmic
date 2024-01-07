package event_plugin

import (
	"encoding/gob"
	"log"
	"net/rpc"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/types"
)

type InitArgs struct {
	Cfg interface{}
}

type ApplyArgs struct {
	Events []*formatters.EventMsg
}

type ApplyResponse struct {
	Events []*formatters.EventMsg
}

type (
	Actionresponse     struct{}
	InitResponse       struct{}
	Targetresponse     struct{}
	Proccessorresponse struct{}
)

type eventProcessorRPCServer struct {
	Impl formatters.EventProcessor
}

func init() {
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}

func (s *eventProcessorRPCServer) Init(args *InitArgs, resp *InitResponse) error {
	return s.Impl.Init(args.Cfg)
}

func (s *eventProcessorRPCServer) Apply(args *ApplyArgs, resp *ApplyResponse) error {
	resp.Events = s.Impl.Apply(args.Events...)
	return nil
}

func (s *eventProcessorRPCServer) WithActions(args map[string]map[string]interface{}, resp *Actionresponse) error {
	s.Impl.WithActions(args)
	return nil
}

func (s *eventProcessorRPCServer) WithTargets(args map[string]*types.TargetConfig, resp *Targetresponse) error {
	s.Impl.WithTargets(args)
	return nil
}

func (s *eventProcessorRPCServer) WithProcessors(
	args map[string]map[string]interface{},
	resp *Proccessorresponse,
) error {
	s.Impl.WithProcessors(args)
	return nil
}

func (s *eventProcessorRPCServer) WithLogger() error {
	return nil
}

type EventProcessorRPC struct{ client *rpc.Client }

func (g *EventProcessorRPC) Init(cfg interface{}, opts ...formatters.Option) error {
	for _, opt := range opts {
		opt(g)
	}
	err := g.client.Call("Plugin.Init", &InitArgs{Cfg: cfg}, &InitResponse{})
	if err != nil {
		return err
	}
	return nil
}

func (g *EventProcessorRPC) Apply(event ...*formatters.EventMsg) []*formatters.EventMsg {
	var resp ApplyResponse
	err := g.client.Call("Plugin.Apply", &ApplyArgs{Events: event}, &resp)
	if err != nil {
		log.Print("RPC client call error: ", err)
		return nil
	}
	return resp.Events
}

func (g *EventProcessorRPC) WithActions(act map[string]map[string]interface{}) {
	err := g.client.Call("Plugin.WithActions", act, &Actionresponse{})
	if err != nil {
		log.Print("RPC client call error: ", err)
	}
}

func (g *EventProcessorRPC) WithTargets(tcs map[string]*types.TargetConfig) {
	err := g.client.Call("Plugin.WithTargets", tcs, &Targetresponse{})
	if err != nil {
		log.Print("RPC client call error: ", err)
	}
}

func (g *EventProcessorRPC) WithProcessors(procs map[string]map[string]any) {
	err := g.client.Call("Plugin.WithProcessors", procs, &Proccessorresponse{})
	if err != nil {
		log.Print("RPC client call error: ", err)
	}
}

func (g *EventProcessorRPC) WithLogger(l *log.Logger) {
}
