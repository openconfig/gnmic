package collector

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/openconfig/gnmi/proto/gnmi"
)

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
	fmt.Printf("%+v\n", v)
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
	spew.Dump(evs)
	t.Logf("%v", evs)
	b, _ := json.MarshalIndent(evs, "", "  ")
	fmt.Println(string(b))
}
