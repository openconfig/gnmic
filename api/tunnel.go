package api

import (
	"fmt"

	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
	"google.golang.org/protobuf/proto"
)

// TunnelOption is a function that acts on the supplied proto.Message.
// The message is expected to be one of the protobuf defined gRPC tunnel messages
// exchanged by the RPCs or any of the nested messages.
type TunnelOption func(proto.Message) error

// apply is a helper function that simply applies the options to the proto.Message.
// It returns an error if any of the options fails.
func applyTunnelOpts(m proto.Message, opts ...TunnelOption) error {
	for _, o := range opts {
		if err := o(m); err != nil {
			return err
		}
	}
	return nil
}

func NewRegisterOpTarget(opts ...TunnelOption) (*tpb.RegisterOp, error) {
	m := &tpb.RegisterOp{
		Registration: new(tpb.RegisterOp_Target),
	}
	err := applyTunnelOpts(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func NewRegisterOpSession(opts ...TunnelOption) (*tpb.RegisterOp, error) {
	m := &tpb.RegisterOp{
		Registration: new(tpb.RegisterOp_Session),
	}
	err := applyTunnelOpts(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func NewRegisterOpSubscription(opts ...TunnelOption) (*tpb.RegisterOp, error) {
	m := &tpb.RegisterOp{
		Registration: new(tpb.RegisterOp_Subscription),
	}
	err := applyTunnelOpts(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func NewData(opts ...TunnelOption) (*tpb.Data, error) {
	m := new(tpb.Data)
	err := applyTunnelOpts(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Messages options

func TunnelTarget(opts ...TunnelOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.RegisterOp:
			switch msg := msg.Registration.(type) {
			case *tpb.RegisterOp_Target:
				target := new(tpb.Target)
				err := applyTunnelOpts(target, opts...)
				if err != nil {
					return err
				}
				msg.Target = target
			}
		default:
			return fmt.Errorf("option TunnelTarget: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func TunnelSession(opts ...TunnelOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.RegisterOp:
			switch msg := msg.Registration.(type) {
			case *tpb.RegisterOp_Session:
				session := new(tpb.Session)
				err := applyTunnelOpts(session, opts...)
				if err != nil {
					return err
				}
				msg.Session = session
			}
		default:
			return fmt.Errorf("option TunnelSession: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func TunnelSubscription(opts ...TunnelOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.RegisterOp:
			switch msg := msg.Registration.(type) {
			case *tpb.RegisterOp_Subscription:
				subscription := new(tpb.Subscription)
				err := applyTunnelOpts(subscription, opts...)
				if err != nil {
					return err
				}
				msg.Subscription = subscription
			}
		default:
			return fmt.Errorf("option TunnelSubscription: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Common Options
func TargetOpRemove() func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Target:
			msg.Op = tpb.Target_REMOVE
		default:
			return fmt.Errorf("option TargetOpRemove: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func Accept(b bool) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Target:
			msg.Accept = b
		case *tpb.Session:
			msg.Accept = b
		case *tpb.Subscription:
			msg.Accept = b
		default:
			return fmt.Errorf("option Accept: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func TargetName(n string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Target:
			msg.Target = n
		case *tpb.Session:
			msg.Target = n
		default:
			return fmt.Errorf("option TargetName: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func TargetType(typ string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Target:
			msg.TargetType = typ
		case *tpb.Session:
			msg.TargetType = typ
		case *tpb.Subscription:
			msg.TargetType = typ
		default:
			return fmt.Errorf("option TargetType: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func Error(e string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Target:
			msg.Error = e
		case *tpb.Session:
			msg.Error = e
		case *tpb.Subscription:
			msg.Error = e
		default:
			return fmt.Errorf("option Error: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Target Options

func TargetOpAdd() func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Target:
			msg.Op = tpb.Target_ADD
		default:
			return fmt.Errorf("option TargetOpAdd: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func Tag(t int32) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Session:
			msg.Tag = t
		case *tpb.Data:
			msg.Tag = t
		default:
			return fmt.Errorf("option Tag: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Subscription Options

func SubscriptionOpSubscribe() func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Subscription:
			msg.Op = tpb.Subscription_SUBCRIBE
			//
		default:
			return fmt.Errorf("option SubscriptionOpSubscribe: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func SubscriptionOpUnsubscribe() func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Subscription:
			msg.Op = tpb.Subscription_UNSUBCRIBE
			//
		default:
			return fmt.Errorf("option SubscriptionOpUnsubscribe: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Data Options

func Data(d []byte) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Data:
			msg.Data = d
		default:
			return fmt.Errorf("option Data: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}
func Close(b bool) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *tpb.Data:
			msg.Close = b
		default:
			return fmt.Errorf("option Close: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}
