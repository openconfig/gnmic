package pipeline

import (
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"google.golang.org/protobuf/proto"
)

// Msg contains the data to be passed from targets or inputs to outputs.
type Msg struct {
	Msg     proto.Message
	Meta    outputs.Meta
	Events  []*formatters.EventMsg
	Outputs map[string]struct{}
}

func NewMsg(msg proto.Message, meta outputs.Meta,
	events []*formatters.EventMsg,
	outputs map[string]struct{}) *Msg {
	return &Msg{
		Msg:     msg,
		Meta:    meta,
		Events:  events,
		Outputs: outputs,
	}
}
