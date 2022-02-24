package api

import (
	"errors"
	"testing"

	"github.com/karimra/gnmic/testutils"
	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
)

type registerOpInput struct {
	opts []TunnelOption
	msg  *tpb.RegisterOp
	err  error
}

var registerOpTargetTestSet = map[string]registerOpInput{
	"target_add": {
		opts: []TunnelOption{
			TunnelTarget(
				TargetOpAdd(),
				Accept(true),
				TargetName("target1"),
				TargetType("target_type1"),
			),
		},
		msg: &tpb.RegisterOp{
			Registration: &tpb.RegisterOp_Target{
				Target: &tpb.Target{
					Op:         tpb.Target_ADD,
					Accept:     true,
					Target:     "target1",
					TargetType: "target_type1",
				},
			}},
		err: nil,
	},
	"target_remove": {
		opts: []TunnelOption{
			TunnelTarget(
				TargetOpRemove(),
				Accept(true),
				TargetName("target1"),
				TargetType("target_type1"),
			),
		},
		msg: &tpb.RegisterOp{
			Registration: &tpb.RegisterOp_Target{
				Target: &tpb.Target{
					Op:         tpb.Target_REMOVE,
					Accept:     true,
					Target:     "target1",
					TargetType: "target_type1",
				},
			}},
		err: nil,
	},
	"target_error": {
		opts: []TunnelOption{
			TunnelTarget(
				TargetOpRemove(),
				Accept(true),
				TargetName("target1"),
				TargetType("target_type1"),
				Error("err1"),
			),
		},
		msg: &tpb.RegisterOp{
			Registration: &tpb.RegisterOp_Target{
				Target: &tpb.Target{
					Op:         tpb.Target_REMOVE,
					Accept:     true,
					Target:     "target1",
					TargetType: "target_type1",
					Error:      "err1",
				},
			}},
		err: nil,
	},
	"target_nok": {
		opts: []TunnelOption{
			TunnelTarget(
				Tag(42),
				Accept(true),
				TargetName("target1"),
				TargetType("target_type1"),
			),
		},
		msg: nil,
		err: ErrInvalidMsgType,
	},
}
var registerOpSessionTestSet = map[string]registerOpInput{
	"session_ok": {
		opts: []TunnelOption{
			TunnelSession(
				Tag(42),
				Accept(true),
				TargetName("target1"),
				TargetType("target_type1"),
			),
		},
		msg: &tpb.RegisterOp{
			Registration: &tpb.RegisterOp_Session{
				Session: &tpb.Session{
					Tag:        42,
					Accept:     true,
					Target:     "target1",
					TargetType: "target_type1",
				},
			}},
		err: nil,
	},
	"session_nok": {
		opts: []TunnelOption{
			TunnelSession(
				TargetOpAdd(),
				Accept(true),
				TargetName("target1"),
				TargetType("target_type1"),
			),
		},
		msg: nil,
		err: ErrInvalidMsgType,
	},
	"session_err": {
		opts: []TunnelOption{
			TunnelSession(
				Tag(42),
				Accept(true),
				TargetName("target1"),
				TargetType("target_type1"),
				Error("err1"),
			),
		},
		msg: &tpb.RegisterOp{
			Registration: &tpb.RegisterOp_Session{
				Session: &tpb.Session{
					Tag:        42,
					Accept:     true,
					Target:     "target1",
					TargetType: "target_type1",
					Error:      "err1",
				},
			}},
		err: nil,
	},
}
var registerOpSubscriptionTestSet = map[string]registerOpInput{
	"subscription_op_subscribe": {
		opts: []TunnelOption{
			TunnelSubscription(
				SubscriptionOpSubscribe(),
				Accept(true),
				TargetType("target_type1"),
			),
		},
		msg: &tpb.RegisterOp{
			Registration: &tpb.RegisterOp_Subscription{
				Subscription: &tpb.Subscription{
					Op:         tpb.Subscription_SUBCRIBE,
					Accept:     true,
					TargetType: "target_type1",
				},
			}},
		err: nil,
	},
	"subscription_op_unsubscribe": {
		opts: []TunnelOption{
			TunnelSubscription(
				SubscriptionOpUnsubscribe(),
				Accept(true),
				TargetType("target_type1"),
			),
		},
		msg: &tpb.RegisterOp{
			Registration: &tpb.RegisterOp_Subscription{
				Subscription: &tpb.Subscription{
					Op:         tpb.Subscription_UNSUBCRIBE,
					Accept:     true,
					TargetType: "target_type1",
				},
			}},
		err: nil,
	},
	"subscription_nok": {
		opts: []TunnelOption{
			TunnelSubscription(
				SubscriptionOpSubscribe(),
				Accept(true),
				TargetName("target1"),
				TargetType("target_type1"),
			),
		},
		msg: nil,
		err: ErrInvalidMsgType,
	},
	"subscription_err": {
		opts: []TunnelOption{
			TunnelSubscription(
				SubscriptionOpUnsubscribe(),
				Accept(true),
				TargetType("target_type1"),
				Error("err1"),
			),
		},
		msg: &tpb.RegisterOp{
			Registration: &tpb.RegisterOp_Subscription{
				Subscription: &tpb.Subscription{
					Op:         tpb.Subscription_UNSUBCRIBE,
					Accept:     true,
					TargetType: "target_type1",
					Error:      "err1",
				},
			}},
		err: nil,
	},
}

func TestNewRegister(t *testing.T) {
	for name, item := range registerOpTargetTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewRegisterOpTarget(item.opts...)
			if err != nil {
				uerr := errors.Unwrap(err)
				if !errors.Is(uerr, item.err) {
					t.Errorf("%q failed", name)
					t.Errorf("%q expected err : %v", name, item.err)
					t.Errorf("%q got err      : %v", name, err)
					t.Fail()
				}
				return
			}
			if !testutils.RegisterOpEqual(nreq, item.msg) {
				t.Errorf("%q failed", name)
				t.Errorf("%q expected result : %+v", name, item.msg)
				t.Errorf("%q got result      : %+v", name, nreq)
				t.Fail()
			}
		})
	}
	for name, item := range registerOpSessionTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewRegisterOpSession(item.opts...)
			if err != nil {
				uerr := errors.Unwrap(err)
				if !errors.Is(uerr, item.err) {
					t.Errorf("%q failed", name)
					t.Errorf("%q expected err : %v", name, item.err)
					t.Errorf("%q got err      : %v", name, err)
					t.Fail()
				}
				return
			}
			if !testutils.RegisterOpEqual(nreq, item.msg) {
				t.Errorf("%q failed", name)
				t.Errorf("%q expected result : %+v", name, item.msg)
				t.Errorf("%q got result      : %+v", name, nreq)
				t.Fail()
			}
		})
	}
	for name, item := range registerOpSubscriptionTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewRegisterOpSubscription(item.opts...)
			if err != nil {
				uerr := errors.Unwrap(err)
				if !errors.Is(uerr, item.err) {
					t.Errorf("%q failed", name)
					t.Errorf("%q expected err : %v", name, item.err)
					t.Errorf("%q got err      : %v", name, err)
					t.Fail()
				}
				return
			}
			if !testutils.RegisterOpEqual(nreq, item.msg) {
				t.Errorf("%q failed", name)
				t.Errorf("%q expected result : %+v", name, item.msg)
				t.Errorf("%q got result      : %+v", name, nreq)
				t.Fail()
			}
		})
	}
}

type dataInput struct {
	opts []TunnelOption
	msg  *tpb.Data
	err  error
}

var dataTestSet = map[string]dataInput{
	"data_ok": {
		opts: []TunnelOption{
			Tag(42),
			Data([]byte("foo")),
			Close(true),
		},
		msg: &tpb.Data{
			Tag:   42,
			Data:  []byte("foo"),
			Close: true,
		},
		err: nil,
	},
	"data_nok": {
		opts: []TunnelOption{
			TargetName("bar"),
			Tag(42),
			Data([]byte("foo")),
			Close(true),
		},
		msg: nil,
		err: ErrInvalidMsgType,
	},
}

func TestNewData(t *testing.T) {
	for name, item := range dataTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewData(item.opts...)
			if err != nil {
				uerr := errors.Unwrap(err)
				if !errors.Is(uerr, item.err) {
					t.Errorf("%q failed", name)
					t.Errorf("%q expected err : %v", name, item.err)
					t.Errorf("%q got err      : %v", name, err)
					t.Fail()
				}
				return
			}
			if !testutils.TunnelDataEqual(nreq, item.msg) {
				t.Errorf("%q failed", name)
				t.Errorf("%q expected result : %+v", name, item.msg)
				t.Errorf("%q got result      : %+v", name, nreq)
				t.Fail()
			}
		})
	}
}
