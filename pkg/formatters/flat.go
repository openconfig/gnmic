// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package formatters

import (
	"errors"
	"path/filepath"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/path"
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
			prefix := path.GnmiPathToXPath(n.GetPrefix(), false)
			for _, u := range n.GetUpdate() {
				p := path.GnmiPathToXPath(u.GetPath(), false)
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
			prefix := path.GnmiPathToXPath(n.GetPrefix(), false)
			for _, u := range n.GetUpdate() {
				p := path.GnmiPathToXPath(u.GetPath(), false)
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
