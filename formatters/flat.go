package formatters

import (
	"errors"
	"path/filepath"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

func ResponsesFlat(msgs ...proto.Message) (map[string]interface{}, error) {
	rs := make(map[string]interface{})
	for _, msg := range msgs {
		mr, err := responseFlat(msg)
		if err != nil {
			return nil, err
		}
		for k, v := range mr {
			rs[k] = v
		}
	}
	return rs, nil
}

func responseFlat(msg proto.Message) (map[string]interface{}, error) {
	switch msg := msg.ProtoReflect().Interface().(type) {
	case *gnmi.GetResponse:
		rs := make(map[string]interface{})
		for _, n := range msg.GetNotification() {
			prefix := gnmiPathToXPath(n.GetPrefix())
			for _, u := range n.GetUpdate() {
				p := gnmiPathToXPath(u.GetPath())
				vmap, err := getValueFlat(filepath.Join(prefix, p), u.GetVal())
				if err != nil {
					return nil, err
				}
				if len(vmap) == 0 {
					rs[p] = "{}"
					continue
				}
				for p, v := range vmap {
					rs[p] = v
				}
			}
		}
		return rs, nil
	case *gnmi.SubscribeResponse:
		rs := make(map[string]interface{})
		n := msg.GetUpdate()
		if n != nil {
			prefix := gnmiPathToXPath(n.GetPrefix())
			for _, u := range n.GetUpdate() {
				p := gnmiPathToXPath(u.GetPath())
				vmap, err := getValueFlat(filepath.Join(prefix, p), u.GetVal())
				if err != nil {
					return nil, err
				}
				if len(vmap) == 0 {
					rs[p] = "{}"
					continue
				}
				for p, v := range vmap {
					rs[p] = v
				}
			}
		}
		return rs, nil
	}
	return nil, errors.New("unsupported message type")
}
