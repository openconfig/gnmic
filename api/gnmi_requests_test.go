package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/karimra/gnmic/testutils"
	"github.com/openconfig/gnmi/proto/gnmi"
)

type getRequestInput struct {
	opts []GNMIOption
	req  *gnmi.GetRequest
}

var getRequestTestSet = map[string]getRequestInput{
	"path": {
		opts: []GNMIOption{
			Path("system/name"),
		},
		req: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{
							Name: "system",
						},
						{
							Name: "name",
						},
					},
				},
			},
		},
	},
	"two_paths": {
		opts: []GNMIOption{
			Path("system/name"),
			Path("system/gnmi-server"),
		},
		req: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{
							Name: "system",
						},
						{
							Name: "name",
						},
					},
				},
				{
					Elem: []*gnmi.PathElem{
						{
							Name: "system",
						},
						{
							Name: "gnmi-server",
						},
					},
				},
			},
		},
	},
	"prefix": {
		opts: []GNMIOption{
			Prefix("system/name"),
		},
		req: &gnmi.GetRequest{
			Prefix: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{
						Name: "system",
					},
					{
						Name: "name",
					},
				},
			},
		},
	},
	"target": {
		opts: []GNMIOption{
			Target("target1"),
		},
		req: &gnmi.GetRequest{
			Prefix: &gnmi.Path{
				Target: "target1",
			},
		},
	},
	"prefix_target_path": {
		opts: []GNMIOption{
			Prefix("system"),
			Path("name"),
			Target("target1"),
		},
		req: &gnmi.GetRequest{
			Prefix: &gnmi.Path{
				Target: "target1",
				Elem: []*gnmi.PathElem{
					{
						Name: "system",
					},
				},
			},
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{
							Name: "name",
						},
					},
				},
			},
		},
	},
	"data_type": {
		opts: []GNMIOption{
			Path("system/name"),
			DataType("config"),
		},
		req: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{
							Name: "system",
						},
						{
							Name: "name",
						},
					},
				},
			},
			Type: gnmi.GetRequest_CONFIG,
		},
	},
	"encoding": {
		opts: []GNMIOption{
			Path("system/name"),
			DataType("config"),
			Encoding("json_ietf"),
		},
		req: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{
							Name: "system",
						},
						{
							Name: "name",
						},
					},
				},
			},
			Type:     gnmi.GetRequest_CONFIG,
			Encoding: gnmi.Encoding_JSON_IETF,
		},
	},
}

func TestNewGetRequest(t *testing.T) {
	for name, item := range getRequestTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewGetRequest(item.opts...)
			if err != nil {
				t.Errorf("failed at %q: %v", name, err)
				t.Fail()
			}
			if !testutils.CompareGetRequests(nreq, item.req) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.req)
				t.Errorf("     got %+v", nreq)
				t.Fail()
			}
		})
	}
}

type setRequestInput struct {
	opts []GNMIOption
	req  *gnmi.SetRequest
}

var setRequestTestSet = map[string]setRequestInput{
	"update": {
		opts: []GNMIOption{
			Update(Path("/system/name/host-name"), Value("srl2", "json_ietf")),
		},
		req: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "name"},
							{Name: "host-name"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl2\"")},
					},
				},
			},
		},
	},
	"two_updates": {
		opts: []GNMIOption{
			Update(
				Path("/system/name/host-name"),
				Value("srl2", "json_ietf"),
			),
			Update(
				Path("/system/gnmi-server/unix-socket/admin-state"),
				Value("enable", "json_ietf"),
			),
		},
		req: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "name"},
							{Name: "host-name"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl2\"")},
					},
				},
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "gnmi-server"},
							{Name: "unix-socket"},
							{Name: "admin-state"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"enable\"")},
					},
				},
			},
		},
	},
	"replace": {
		opts: []GNMIOption{
			Replace(Path("/system/name/host-name"), Value("srl2", "json_ietf")),
		},
		req: &gnmi.SetRequest{
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "name"},
							{Name: "host-name"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl2\"")},
					},
				},
			},
		},
	},
	"two_replaces": {
		opts: []GNMIOption{
			Replace(
				Path("/system/name/host-name"),
				Value("srl2", "json_ietf"),
			),
			Replace(
				Path("/system/gnmi-server/unix-socket/admin-state"),
				Value("enable", "json_ietf"),
			),
		},
		req: &gnmi.SetRequest{
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "name"},
							{Name: "host-name"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl2\"")},
					},
				},
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "gnmi-server"},
							{Name: "unix-socket"},
							{Name: "admin-state"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"enable\"")},
					},
				},
			},
		},
	},
	"delete": {
		opts: []GNMIOption{
			Delete("/system/name/host-name"),
		},
		req: &gnmi.SetRequest{
			Delete: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "system"},
						{Name: "name"},
						{Name: "host-name"},
					},
				},
			},
		},
	},
	"two_deletes": {
		opts: []GNMIOption{
			Delete("/system/name/host-name"),
			Delete("interface/description"),
		},
		req: &gnmi.SetRequest{
			Delete: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "system"},
						{Name: "name"},
						{Name: "host-name"},
					},
				},
				{
					Elem: []*gnmi.PathElem{
						{Name: "interface"},
						{Name: "description"},
					},
				},
			},
		},
	},
	"update_replace": {
		opts: []GNMIOption{
			Update(
				Path("/system/name/host-name"),
				Value("srl2", "json_ietf"),
			),
			Replace(
				Path("/system/gnmi-server/unix-socket/admin-state"),
				Value("enable", "json_ietf"),
			),
		},
		req: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "name"},
							{Name: "host-name"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl2\"")},
					},
				},
			},
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "gnmi-server"},
							{Name: "unix-socket"},
							{Name: "admin-state"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"enable\"")},
					},
				},
			},
		},
	},
	"update_replace_delete": {
		opts: []GNMIOption{
			Update(
				Path("/system/name/host-name"),
				Value("srl2", "json_ietf"),
			),
			Replace(
				Path("/system/gnmi-server/unix-socket/admin-state"),
				Value("enable", "json_ietf"),
			),
			Delete("/system/name/host-name"),
		},
		req: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "name"},
							{Name: "host-name"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl2\"")},
					},
				},
			},
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "gnmi-server"},
							{Name: "unix-socket"},
							{Name: "admin-state"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"enable\"")},
					},
				},
			},
			Delete: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "system"},
						{Name: "name"},
						{Name: "host-name"},
					},
				},
			},
		},
	},
}

func TestNewSetRequest(t *testing.T) {
	for name, item := range setRequestTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewSetRequest(item.opts...)
			if err != nil {
				t.Errorf("failed at %q: %v", name, err)
				t.Fail()
			}
			if !testutils.CompareSetRequests(nreq, item.req) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.req)
				t.Errorf("     got %+v", nreq)
				t.Fail()
			}
		})
	}
}

type subscribeRequestInput struct {
	opts []GNMIOption
	req  *gnmi.SubscribeRequest
}

var subscribeRequestTestSet = map[string]subscribeRequestInput{
	"subscription": {
		opts: []GNMIOption{
			Subscription(
				Path("system/name"),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Subscription: []*gnmi.Subscription{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{Name: "system"},
									{Name: "name"},
								},
							},
						},
					},
				},
			},
		},
	},
	"subscription_list_mode": {
		opts: []GNMIOption{
			SubscriptionListMode("once"),
			Subscription(
				Path("system/name"),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Mode: gnmi.SubscriptionList_ONCE,
					Subscription: []*gnmi.Subscription{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{Name: "system"},
									{Name: "name"},
								},
							},
						},
					},
				},
			},
		},
	},
	"subscription_mode": {
		opts: []GNMIOption{
			Subscription(
				Path("system/name"),
				SubscriptionMode("sample"),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Subscription: []*gnmi.Subscription{
						{
							Mode: gnmi.SubscriptionMode_SAMPLE,
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{Name: "system"},
									{Name: "name"},
								},
							},
						},
					},
				},
			},
		},
	},
	"subscription_sample": {
		opts: []GNMIOption{
			Subscription(
				Path("system/name"),
				SubscriptionMode("sample"),
				SampleInterval(10*time.Second),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Subscription: []*gnmi.Subscription{
						{
							Mode:           gnmi.SubscriptionMode_SAMPLE,
							SampleInterval: uint64(10 * time.Second),
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{Name: "system"},
									{Name: "name"},
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestNewSubscribeRequest(t *testing.T) {
	for name, item := range subscribeRequestTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewSubscribeRequest(item.opts...)
			if err != nil {
				t.Errorf("failed at %q: %v", name, err)
				t.Fail()
			}
			if !testutils.CompareSubscribeRequests(nreq, item.req) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.req)
				t.Errorf("     got %+v", nreq)
				t.Fail()
			}
		})
	}
}

// TODO: Value tests

type valueInput struct {
	data     interface{}
	encoding string
	msg      *gnmi.Update
}

var valueTestSet = map[string]valueInput{
	// json
	"json_string": {
		data:     "value",
		encoding: "json",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: []byte("\"value\""),
				},
			},
		},
	},
	"json_string_array": {
		data:     []string{"foo", "bar"},
		encoding: "json",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: []byte("[\"foo\",\"bar\"]"),
				},
			},
		},
	},
	"json_interface{}_array": {
		data:     []interface{}{"foo", 2},
		encoding: "json",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: []byte("[\"foo\",2]"),
				},
			},
		},
	},
	"json_map": {
		data:     map[string]interface{}{"k": "v"},
		encoding: "json",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: []byte("{\"k\":\"v\"}"),
				},
			},
		},
	},
	// json_ietf
	"json_ietf_string": {
		data:     "value",
		encoding: "json_ietf",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: []byte("\"value\""),
				},
			},
		},
	},
	"json_ietf_string_array": {
		data:     []string{"foo", "bar"},
		encoding: "json_ietf",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: []byte("[\"foo\",\"bar\"]"),
				},
			},
		},
	},
	"json_ietf_interface{}_array": {
		data:     []interface{}{"foo", 2},
		encoding: "json_ietf",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: []byte("[\"foo\",2]"),
				},
			},
		},
	},
	"json_ietf_map": {
		data:     map[string]interface{}{"k": "v"},
		encoding: "json_ietf",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: []byte("{\"k\":\"v\"}"),
				},
			},
		},
	},
	// ascii
	"ascii_string": {
		data:     "foo",
		encoding: "ascii",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{
					AsciiVal: "foo",
				},
			},
		},
	},
	"ascii_string_array": {
		data:     []string{"foo", "bar"},
		encoding: "ascii",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_LeaflistVal{
					LeaflistVal: &gnmi.ScalarArray{
						Element: []*gnmi.TypedValue{
							{
								Value: &gnmi.TypedValue_StringVal{StringVal: "foo"},
							},
							{
								Value: &gnmi.TypedValue_StringVal{StringVal: "bar"},
							},
						},
					},
				},
			},
		},
	},
	"ascii_interface{}_array": {
		data:     []interface{}{"foo", 42},
		encoding: "ascii",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_LeaflistVal{
					LeaflistVal: &gnmi.ScalarArray{
						Element: []*gnmi.TypedValue{
							{
								Value: &gnmi.TypedValue_StringVal{StringVal: "foo"},
							},
							{
								Value: &gnmi.TypedValue_IntVal{IntVal: 42},
							},
						},
					},
				},
			},
		},
	},
}

func TestValue(t *testing.T) {
	for name, item := range valueTestSet {
		t.Run(name, func(t *testing.T) {
			upd := new(gnmi.Update)
			err := Value(item.data, item.encoding)(upd)
			if err != nil {
				t.Errorf("failed at %q with error: %v", name, err)
				t.Fail()
			}
			fmt.Println(upd.GetVal().GetValue())
			if !cmp.Equal(item.msg.GetVal().GetValue(), upd.GetVal().GetValue(),
				cmpopts.IgnoreUnexported(gnmi.TypedValue{}, gnmi.ScalarArray{})) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.msg.GetVal().GetValue())
				t.Errorf("     got %+v", upd.GetVal().GetValue())
				t.Fail()
			}
		})
	}
}
