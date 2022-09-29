// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package outputs

import (
	"google.golang.org/protobuf/proto"
)

type ProtoMsg struct {
	m    proto.Message
	meta Meta
}

func NewProtoMsg(m proto.Message, meta Meta) *ProtoMsg {
	return &ProtoMsg{
		m:    m,
		meta: meta,
	}
}

func (m *ProtoMsg) GetMsg() proto.Message {
	if m == nil {
		return nil
	}
	return m.m
}

func (m *ProtoMsg) GetMeta() Meta {
	if m == nil {
		return nil
	}
	return m.meta
}
