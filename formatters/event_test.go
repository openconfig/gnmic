package formatters

import (
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
)

type item struct {
	ev *EventMsg
	m  map[string]interface{}
}

var eventMsgtestSet = map[string][]item{
	"nil": {
		{
			ev: nil,
			m:  nil,
		},
		{
			ev: new(EventMsg),
			m:  make(map[string]interface{}),
		},
	},
	"filled": {
		{
			ev: &EventMsg{
				Timestamp: 100,
				Values:    map[string]interface{}{"value1": int64(1)},
				Tags:      map[string]string{"tag1": "1"},
			},
			m: map[string]interface{}{
				"timestamp": int64(100),
				"values": map[string]interface{}{
					"value1": int64(1),
				},
				"tags": map[string]interface{}{
					"tag1": "1",
				},
			},
		},
		{
			ev: &EventMsg{
				Name:      "sub1",
				Timestamp: 100,
				Tags: map[string]string{
					"tag1": "1",
					"tag2": "1",
				},
			},
			m: map[string]interface{}{
				"name":      "sub1",
				"timestamp": int64(100),
				"tags": map[string]interface{}{
					"tag1": "1",
					"tag2": "1",
				},
			},
		},
		{
			ev: &EventMsg{
				Name:      "sub1",
				Timestamp: 100,
				Values: map[string]interface{}{
					"value1": int64(1),
					"value2": int64(1),
				},
				Tags: map[string]string{
					"tag1": "1",
					"tag2": "1",
				},
			},
			m: map[string]interface{}{
				"name":      "sub1",
				"timestamp": int64(100),
				"values": map[string]interface{}{
					"value1": int64(1),
					"value2": int64(1),
				},
				"tags": map[string]interface{}{
					"tag1": "1",
					"tag2": "1",
				},
			},
		},
	},
}

func TestToMap(t *testing.T) {
	for name, items := range eventMsgtestSet {
		for i, item := range items {
			t.Run(name, func(t *testing.T) {
				t.Logf("running test item %d", i)
				out := item.ev.ToMap()
				if !reflect.DeepEqual(out, item.m) {
					t.Logf("failed at %q item %d", name, i)
					t.Logf("expected: (%T)%+v", item.m, item.m)
					t.Logf("     got: (%T)%+v", out, out)
					t.Fail()
				}
			})
		}
	}
}

func TestFromMap(t *testing.T) {
	for name, items := range eventMsgtestSet {
		for i, item := range items {
			t.Run(name, func(t *testing.T) {
				t.Logf("running test item %d", i)
				out, err := EventFromMap(item.m)
				if err != nil {
					t.Logf("failed at %q: %v", name, err)
					t.Fail()
				}
				if !reflect.DeepEqual(out, item.ev) {
					t.Logf("failed at %q item %d", name, i)
					t.Logf("expected: (%T)%+v", item.m, item.m)
					t.Logf("     got: (%T)%+v", out, out)
					t.Fail()
				}
			})
		}
	}
}

var jsonData = `
{
    "admin-state": "enable",
    "ipv4": {
        "primary": {
            "address": "1.1.1.1",
            "prefix-length": 32
        }
    }
}
`
var value = &gnmi.TypedValue{
	Value: &gnmi.TypedValue_JsonVal{
		JsonVal: []byte(jsonData),
	},
}

func TestGetValueFlat(t *testing.T) {
	v, err := getValueFlat("/configure/router[router-name=Base]/interface[interface-name=int1]", value)
	if err != nil {
		t.Error(err)
	}
	//fmt.Printf("%+v\n", v)
	t.Logf("%v", v)
}

func TestResponseToEventMsgs(t *testing.T) {
	rsp := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Timestamp: time.Now().UnixNano(),
				Prefix: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "a"},
					},
				},
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{
							Elem: []*gnmi.PathElem{
								{Name: "b",
									Key: map[string]string{
										"k1": "v1",
									},
								},
								{Name: "c",
									Key: map[string]string{
										"k2": "v2",
									}},
							},
						},
						Val: &gnmi.TypedValue{
							Value: &gnmi.TypedValue_StringVal{
								StringVal: "value",
							},
						},
					},
				},
			},
		},
	}
	evs, err := ResponseToEventMsgs("subname", rsp, map[string]string{"k1": "v0"})
	if err != nil {
		t.Error(err)
	}
	t.Logf("%v", evs)
	//b, _ := json.MarshalIndent(evs, "", "  ")
	//fmt.Println(string(b))
}
