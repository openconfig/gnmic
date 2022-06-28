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
