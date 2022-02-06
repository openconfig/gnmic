package api

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/karimra/gnmic/testutils"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
)

//  Capabilities Request / Response tests
func TestNewCapabilitiesRequest(t *testing.T) {
	name := "single_case"
	t.Run(name, func(t *testing.T) {
		nreq, err := NewCapabilitiesRequest()
		if err != nil {
			t.Errorf("failed at %q: %v", name, err)
			t.Fail()
		}
		if !reflect.DeepEqual(new(gnmi.CapabilityRequest), nreq) {
			t.Errorf("failed at %q", name)
			t.Errorf("expected %+v", &gnmi.CapabilityRequest{})
			t.Errorf("     got %+v", nreq)
			t.Fail()
		}
	})
}

type capResponseInput struct {
	opts []GNMIOption
	req  *gnmi.CapabilityResponse
	err  error
}

var capResponseTestSet = map[string]capResponseInput{
	"simple": {
		opts: []GNMIOption{
			SupportedEncoding("json", "json_ietf"),
		},
		req: &gnmi.CapabilityResponse{
			SupportedEncodings: []gnmi.Encoding{
				gnmi.Encoding_JSON,
				gnmi.Encoding_JSON_IETF,
			},
			GNMIVersion: DefaultGNMIVersion,
		},
		err: nil,
	},
	"custom_version": {
		opts: []GNMIOption{
			Version("1.0.0"),
			SupportedEncoding("json", "json_ietf"),
		},
		req: &gnmi.CapabilityResponse{
			SupportedEncodings: []gnmi.Encoding{
				gnmi.Encoding_JSON,
				gnmi.Encoding_JSON_IETF,
			},
			GNMIVersion: "1.0.0",
		},
		err: nil,
	},
	"unsupported_encoding": {
		opts: []GNMIOption{
			Version(DefaultGNMIVersion),
			SupportedEncoding("not_json", "json_ietf"),
		},
		req: &gnmi.CapabilityResponse{
			SupportedEncodings: []gnmi.Encoding{
				gnmi.Encoding_JSON,
				gnmi.Encoding_JSON_IETF,
			},
			GNMIVersion: DefaultGNMIVersion,
		},
		err: ErrInvalidValue,
	},
}

func TestNewCapabilitiesResponse(t *testing.T) {
	for name, item := range capResponseTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewCapabilitiesResponse(item.opts...)
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
			if !testutils.CapabilitiesResponsesEqual(nreq, item.req) {
				t.Errorf("%q failed", name)
				t.Errorf("%q expected result : %+v", name, item.req)
				t.Errorf("%q got result      : %+v", name, nreq)
				t.Fail()
			}
		})
	}
}

// Get Request / Response tests
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
	"extension": {
		opts: []GNMIOption{
			Path("system/name"),
			Extension(&gnmi_ext.Extension{Ext: &gnmi_ext.Extension_History{}}),
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
			Extension: []*gnmi_ext.Extension{
				{Ext: &gnmi_ext.Extension_History{}},
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
	"data_type_ALL": {
		opts: []GNMIOption{
			Path("system/name"),
			DataTypeALL(),
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
			Type: gnmi.GetRequest_ALL,
		},
	},
	"data_type_CONFIG": {
		opts: []GNMIOption{
			Path("system/name"),
			DataTypeCONFIG(),
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
	"data_type_STATE": {
		opts: []GNMIOption{
			Path("system/name"),
			DataTypeSTATE(),
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
			Type: gnmi.GetRequest_STATE,
		},
	},
	"data_type_OPERATIONAL": {
		opts: []GNMIOption{
			Path("system/name"),
			DataTypeOPERATIONAL(),
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
			Type: gnmi.GetRequest_OPERATIONAL,
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
	"encoding_custom": {
		opts: []GNMIOption{
			Path("system/name"),
			DataType("config"),
			EncodingCustom(42),
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
			Encoding: gnmi.Encoding(42),
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
			if !testutils.GetRequestsEqual(nreq, item.req) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.req)
				t.Errorf("     got %+v", nreq)
				t.Fail()
			}
		})
	}
}

type getResponseInput struct {
	opts []GNMIOption
	req  *gnmi.GetResponse
}

var getResponseTestSet = map[string]getResponseInput{
	"simple": {
		opts: []GNMIOption{
			Notification(
				Timestamp(42),
				Update(
					Path("/system/name"),
					Value("srl1", "json_ietf"),
				),
			),
		},
		req: &gnmi.GetResponse{
			Notification: []*gnmi.Notification{
				{
					Timestamp: 42,
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{Name: "system"},
									{Name: "name"},
								},
							},

							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonIetfVal{
									JsonIetfVal: []byte("\"srl1\""),
								},
							},
						},
					},
				},
			},
		},
	},
	"two_updates": {
		opts: []GNMIOption{
			Notification(
				Timestamp(42),
				Update(
					Path("/system/name"),
					Value("srl1", "json_ietf"),
				),
				Update(
					Path("/interface"),
					Value(map[string]interface{}{
						"name": "ethernet-1/1",
					}, "json_ietf"),
				),
			),
		},
		req: &gnmi.GetResponse{
			Notification: []*gnmi.Notification{
				{
					Timestamp: 42,
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{Name: "system"},
									{Name: "name"},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonIetfVal{
									JsonIetfVal: []byte("\"srl1\""),
								},
							},
						},
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{Name: "interface"},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonIetfVal{
									JsonIetfVal: []byte(`{"name":"ethernet-1/1"}`),
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestNewGetResponse(t *testing.T) {
	for name, item := range getResponseTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewGetResponse(item.opts...)
			if err != nil {
				t.Errorf("failed at %q: %v", name, err)
				t.Fail()
			}
			if !testutils.GetResponsesEqual(nreq, item.req) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.req)
				t.Errorf("     got %+v", nreq)
				t.Fail()
			}
		})
	}
}

// Set Request / Response tests
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
			if !testutils.SetRequestsEqual(nreq, item.req) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.req)
				t.Errorf("     got %+v", nreq)
				t.Fail()
			}
		})
	}
}

type setResponseInput struct {
	opts []GNMIOption
	req  *gnmi.SetResponse
}

var setResponseTestSet = map[string]setResponseInput{
	"simple": {
		opts: []GNMIOption{
			Timestamp(42),
			UpdateResult(
				Operation("update"),
				Path("interface"),
			),
		},
		req: &gnmi.SetResponse{
			Response: []*gnmi.UpdateResult{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "interface"},
						},
					},
					Op: gnmi.UpdateResult_UPDATE,
				},
			},
			Timestamp: 42,
		},
	},
	"combined": {
		opts: []GNMIOption{
			Timestamp(42),
			UpdateResult(
				Operation("update"),
				Path("interface"),
			),
			UpdateResult(
				Operation("replace"),
				Path("network-instance"),
			),
		},
		req: &gnmi.SetResponse{
			Response: []*gnmi.UpdateResult{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "interface"},
						},
					},
					Op: gnmi.UpdateResult_UPDATE,
				},
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "network-instance"},
						},
					},
					Op: gnmi.UpdateResult_REPLACE,
				},
			},
			Timestamp: 42,
		},
	},
}

func TestNewSetResponse(t *testing.T) {
	for name, item := range setResponseTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewSetResponse(item.opts...)
			if err != nil {
				t.Errorf("failed at %q: %v", name, err)
				t.Fail()
			}
			if !testutils.SetResponsesEqual(nreq, item.req) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.req)
				t.Errorf("     got %+v", nreq)
				t.Fail()
			}
		})
	}
}

// Subscribe Request / Response tests
type subscribeRequestInput struct {
	opts []GNMIOption
	req  *gnmi.SubscribeRequest
}

var subscribeRequestTestSet = map[string]subscribeRequestInput{
	"subscription": {
		opts: []GNMIOption{
			EncodingJSON_IETF(),
			Subscription(
				Path("system/name"),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Encoding: gnmi.Encoding_JSON_IETF,
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
	"subscription_list_mode_ONCE": {
		opts: []GNMIOption{
			SubscriptionListModeONCE(),
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
	"subscription_list_mode_POLL": {
		opts: []GNMIOption{
			SubscriptionListModePOLL(),
			Subscription(
				Path("system/name"),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Mode: gnmi.SubscriptionList_POLL,
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
	"subscription_list_mode_STREAM": {
		opts: []GNMIOption{
			SubscriptionListModeSTREAM(),
			Subscription(
				Path("system/name"),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Mode: gnmi.SubscriptionList_STREAM,
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
	"subscription_mode_SAMPLE": {
		opts: []GNMIOption{
			Subscription(
				Path("system/name"),
				SubscriptionModeSAMPLE(),
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
	"subscription_mode_TARGET_DEFINED": {
		opts: []GNMIOption{
			Subscription(
				Path("system/name"),
				SubscriptionModeTARGET_DEFINED(),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Subscription: []*gnmi.Subscription{
						{
							Mode: gnmi.SubscriptionMode_TARGET_DEFINED,
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
	"subscription_mode_ON_CHANGE": {
		opts: []GNMIOption{
			Subscription(
				Path("system/name"),
				SubscriptionModeON_CHANGE(),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Subscription: []*gnmi.Subscription{
						{
							Mode: gnmi.SubscriptionMode_ON_CHANGE,
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
			Encoding("json_ietf"),
			Subscription(
				Path("system/name"),
				SubscriptionMode("sample"),
				SampleInterval(10*time.Second),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Encoding: gnmi.Encoding_JSON_IETF,
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
	"subscription_list_encoding_json": {
		opts: []GNMIOption{
			EncodingJSON(),
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
	"subscription_list_encoding_bytes": {
		opts: []GNMIOption{
			EncodingBYTES(),
			Subscription(
				Path("system/name"),
				SubscriptionMode("sample"),
				SampleInterval(10*time.Second),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Encoding: gnmi.Encoding_BYTES,
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
	"subscription_list_encoding_proto": {
		opts: []GNMIOption{
			EncodingPROTO(),
			Subscription(
				Path("system/name"),
				SubscriptionMode("sample"),
				SampleInterval(10*time.Second),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Encoding: gnmi.Encoding_PROTO,
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
	"subscription_list_encoding_ascii": {
		opts: []GNMIOption{
			EncodingASCII(),
			Subscription(
				Path("system/name"),
				SubscriptionMode("sample"),
				SampleInterval(10*time.Second),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Encoding: gnmi.Encoding_ASCII,
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
	"subscription_list_encoding_json_ietf": {
		opts: []GNMIOption{
			EncodingJSON_IETF(),
			Subscription(
				Path("system/name"),
				SubscriptionMode("sample"),
				SampleInterval(10*time.Second),
			),
		},
		req: &gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Subscribe{
				Subscribe: &gnmi.SubscriptionList{
					Encoding: gnmi.Encoding_JSON_IETF,
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
}

func TestNewSubscribeRequest(t *testing.T) {
	for name, item := range subscribeRequestTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewSubscribeRequest(item.opts...)
			if err != nil {
				t.Errorf("failed at %q: %v", name, err)
				t.Fail()
			}
			if !testutils.SubscribeRequestsEqual(nreq, item.req) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.req)
				t.Errorf("     got %+v", nreq)
				t.Fail()
			}
		})
	}
}

type subscribeResponseInput struct {
	opts []GNMIOption
	req  *gnmi.SubscribeResponse
}

var subscribeResponseTestSet = map[string]subscribeResponseInput{
	"simple": {
		opts: []GNMIOption{
			Notification(
				Timestamp(42),
				Alias("alias1"),
				Update(
					Path("interface"),
					Value(map[string]interface{}{
						"name": "ethernet-1/1",
					}, "json_ietf"),
				),
				Delete("/interface[name=ethernet-1/2]"),
				Atomic(true),
			),
		},
		req: &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{
				Update: &gnmi.Notification{
					Timestamp: 42,
					Alias:     "alias1",
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{Name: "interface"},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonIetfVal{
									JsonIetfVal: []byte(`{"name":"ethernet-1/1"}`),
								},
							},
						},
					},
					Delete: []*gnmi.Path{
						{
							Elem: []*gnmi.PathElem{
								{
									Name: "interface",
									Key:  map[string]string{"name": "ethernet-1/2"},
								},
							},
						},
					},
					Atomic: true,
				},
			},
		},
	},
}

func TestNewSubscribeResponse(t *testing.T) {
	for name, item := range subscribeResponseTestSet {
		t.Run(name, func(t *testing.T) {
			nreq, err := NewSubscribeResponse(item.opts...)
			if err != nil {
				t.Errorf("failed at %q: %v", name, err)
				t.Fail()
			}
			if !testutils.SubscribeResponsesEqual(nreq, item.req) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.req)
				t.Errorf("     got %+v", nreq)
				t.Fail()
			}
		})
	}
}

func TestNewSubscribeRequestPoll(t *testing.T) {
	name := "single_case"
	t.Run(name, func(t *testing.T) {
		nreq, err := NewSubscribePollRequest()
		if err != nil {
			t.Errorf("failed at %q: %v", name, err)
			t.Fail()
		}
		if !reflect.DeepEqual(&gnmi.SubscribeRequest{
			Request: &gnmi.SubscribeRequest_Poll{
				Poll: new(gnmi.Poll),
			}}, nreq) {
			t.Errorf("failed at %q", name)
			t.Errorf("expected %+v", &gnmi.SubscribeRequest{Request: &gnmi.SubscribeRequest_Poll{}})
			t.Errorf("     got %+v", nreq)
			t.Fail()
		}
	})
}

func TestNewSubscribeResponseSync(t *testing.T) {
	name := "single_case"
	t.Run(name, func(t *testing.T) {
		nreq, err := NewSubscribeSyncResponse()
		if err != nil {
			t.Errorf("failed at %q: %v", name, err)
			t.Fail()
		}
		if !reflect.DeepEqual(&gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_SyncResponse{
				SyncResponse: true,
			},
		}, nreq) {
			t.Errorf("failed at %q", name)
			t.Errorf("expected %+v", &gnmi.SubscribeRequest{Request: &gnmi.SubscribeRequest_Poll{}})
			t.Errorf("     got %+v", nreq)
			t.Fail()
		}
	})
}

// Value tests
type valueInput struct {
	data     interface{}
	encoding string
	msg      *gnmi.Update
	err      error
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
		data:     []interface{}{"foo", 42},
		encoding: "json",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: []byte("[\"foo\",42]"),
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
		data:     []interface{}{"foo", int(42)},
		encoding: "json_ietf",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: []byte("[\"foo\",42]"),
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
	// typed values
	"typed_value": {
		data: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl1\"")}},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl1\"")}},
		},
		err: nil,
	},
	"typed_value_json": {
		data: &gnmi.TypedValue_JsonVal{JsonVal: []byte("\"srl1\"")},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{JsonVal: []byte("\"srl1\"")}},
		},
		err: nil,
	},
	"typed_value_json_ietf": {
		data: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl1\"")},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("\"srl1\"")}},
		},
		err: nil,
	},
	"typed_value_ascii": {
		data: &gnmi.TypedValue_AsciiVal{AsciiVal: "srl1"},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "srl1"}},
		},
		err: nil,
	},
	"typed_value_bool": {
		data: &gnmi.TypedValue_BoolVal{BoolVal: true},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_BoolVal{BoolVal: true}},
		},
		err: nil,
	},
	"typed_value_bytes": {
		data: &gnmi.TypedValue_BytesVal{BytesVal: []byte{0, 42}},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte{0, 42}}},
		},
		err: nil,
	},
	"typed_value_decimal": {
		data: &gnmi.TypedValue_DecimalVal{DecimalVal: &gnmi.Decimal64{Digits: 420, Precision: 1}},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_DecimalVal{DecimalVal: &gnmi.Decimal64{Digits: 420, Precision: 1}}},
		},
		err: nil,
	},
	"typed_value_float": {
		data: &gnmi.TypedValue_FloatVal{FloatVal: 42.1},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_FloatVal{FloatVal: 42.1}},
		},
		err: nil,
	},
	"typed_value_int": {
		data: &gnmi.TypedValue_IntVal{IntVal: 42},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 42}},
		},
		err: nil,
	},
	"typed_value_uint": {
		data: &gnmi.TypedValue_UintVal{UintVal: 42},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_UintVal{UintVal: 42}},
		},
		err: nil,
	},
	"typed_value_string": {
		data: &gnmi.TypedValue_StringVal{StringVal: "foo"},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "foo"}},
		},
		err: nil,
	},
	"typed_value_leaf_list": {
		data: &gnmi.TypedValue_LeaflistVal{
			LeaflistVal: &gnmi.ScalarArray{
				Element: []*gnmi.TypedValue{
					{Value: &gnmi.TypedValue_StringVal{StringVal: "foo"}},
					{Value: &gnmi.TypedValue_UintVal{UintVal: 42}},
				},
			},
		},
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_LeaflistVal{
					LeaflistVal: &gnmi.ScalarArray{
						Element: []*gnmi.TypedValue{
							{Value: &gnmi.TypedValue_StringVal{StringVal: "foo"}},
							{Value: &gnmi.TypedValue_UintVal{UintVal: 42}},
						},
					},
				},
			},
		},
		err: nil,
	},
	// scalar
	"from_scalar": {
		data:     42,
		encoding: "json",
		msg: &gnmi.Update{
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_IntVal{
					IntVal: 42,
				},
			},
		},
	},
	"invalid_value": {
		data: nil,
		err:  ErrInvalidValue,
	},
}

func TestValue(t *testing.T) {
	for name, item := range valueTestSet {
		t.Run(name, func(t *testing.T) {
			upd := new(gnmi.Update)
			err := Value(item.data, item.encoding)(upd)
			if err != nil {
				uerr := errors.Unwrap(err)
				if !errors.Is(uerr, item.err) {
					t.Errorf("failed at %q with error: %v", name, err)
					t.Errorf("expected err: %+v", item.err)
					t.Errorf("     got err: %+v", err)
					t.Fail()
				}
				return
			}
			if !testutils.GnmiValuesEqual(item.msg.GetVal(), upd.GetVal()) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.msg.GetVal())
				t.Errorf("     got %+v", upd.GetVal())
				t.Fail()
			}
		})
	}
}

// Version tests

func TestVersion(t *testing.T) {
	name := "nil_msg"
	t.Run(name, func(t *testing.T) {
		err := Version(DefaultGNMIVersion)(nil)
		if err != nil {
			if !strings.Contains(err.Error(), ErrInvalidMsgType.Error()) {
				t.Errorf("failed at %q with error: %v", name, err)
				t.Fail()
			}
		}
	})
	name = "invalid_msg"
	t.Run(name, func(t *testing.T) {
		err := Version(DefaultGNMIVersion)(new(gnmi.GetRequest))
		if err != nil {
			if !strings.Contains(err.Error(), ErrInvalidMsgType.Error()) {
				t.Errorf("failed at %q with error: %v", name, err)
				t.Fail()
			}
		}
	})
}

func TestSupportedEncoding(t *testing.T) {
	name := "nil_msg"
	t.Run(name, func(t *testing.T) {
		err := SupportedEncoding("json")(nil)
		if err != nil {
			if !strings.Contains(err.Error(), ErrInvalidMsgType.Error()) {
				t.Errorf("failed at %q with error: %v", name, err)
				t.Fail()
			}
		}
	})
	name = "invalid_msg"
	t.Run(name, func(t *testing.T) {
		err := SupportedEncoding("json")(new(gnmi.GetRequest))
		if err != nil {
			if !strings.Contains(err.Error(), ErrInvalidMsgType.Error()) {
				t.Errorf("failed at %q with error: %v", name, err)
				t.Fail()
			}
		}
	})
	name = "invalid_value"
	t.Run(name, func(t *testing.T) {
		err := SupportedEncoding("not_valid")(new(gnmi.GetRequest))
		if err != nil {
			if !strings.Contains(err.Error(), ErrInvalidMsgType.Error()) {
				t.Errorf("failed at %q with error: %v", name, err)
				t.Fail()
			}
		}
	})
}

func TestSupportedModel(t *testing.T) {
	name := "nil_msg"
	t.Run(name, func(t *testing.T) {
		err := SupportedModel("", "", "")(nil)
		if err != nil {
			if !strings.Contains(err.Error(), ErrInvalidMsgType.Error()) {
				t.Errorf("failed at %q with error: %v", name, err)
				t.Fail()
			}
		}
	})
	name = "invalid_msg"
	t.Run(name, func(t *testing.T) {
		err := SupportedModel("", "", "")(new(gnmi.GetRequest))
		if err != nil {
			if !strings.Contains(err.Error(), ErrInvalidMsgType.Error()) {
				t.Errorf("failed at %q with error: %v", name, err)
				t.Fail()
			}
		}
	})
	name = "ok"
	t.Run(name, func(t *testing.T) {
		capRsp := new(gnmi.CapabilityResponse)
		err := SupportedModel("foo", "bar", "v2")(capRsp)
		if err != nil {
			if !strings.Contains(err.Error(), ErrInvalidMsgType.Error()) {
				t.Errorf("failed at %q with error: %v", name, err)
				t.Fail()
			}
		}
		if len(capRsp.SupportedModels) != 1 {
			t.Fail()
		}
		if capRsp.SupportedModels[0].Name != "foo" {
			t.Fail()
		}
		if capRsp.SupportedModels[0].Organization != "bar" {
			t.Fail()
		}
		if capRsp.SupportedModels[0].Version != "v2" {
			t.Fail()
		}
	})
}
