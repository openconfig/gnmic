package event_jq

import (
	"github.com/karimra/gnmic/formatters"
)

const (
	processorType = "event-jq"
	loggingPrefix = "[" + processorType + "] "
)

// jq runs a jq expression on the received event messages
type jq struct {
	formatters.EventProcessor
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &jq{}
	})
}
