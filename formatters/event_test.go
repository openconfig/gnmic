package formatters

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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
}

func TestTagsFromGNMIPath(t *testing.T) {
	type args struct {
		p *gnmi.Path
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 map[string]string
	}{
		{
			name:  "nil",
			args:  args{p: nil},
			want:  "",
			want1: nil,
		},
		{
			name: "path_no_keys",
			args: args{p: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{
						Name: "interface",
					},
					{
						Name: "statistics",
					},
				},
			}},
			want:  "/interface/statistics",
			want1: make(map[string]string),
		},
		{
			name: "path_with_keys",
			args: args{p: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{
						Name: "interface",
						Key: map[string]string{
							"name": "ethernet-1/1",
						},
					},
					{
						Name: "statistics",
					},
				},
			}},
			want: "/interface/statistics",
			want1: map[string]string{
				"interface_name": "ethernet-1/1",
			},
		},
		{
			name: "path_with_multiple_keys",
			args: args{p: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{
						Name: "elem1",
						Key: map[string]string{
							"bar": "bar_val",
							"foo": "foo_val",
						},
					},
					{
						Name: "elem2",
					},
				},
			}},
			want: "/elem1/elem2",
			want1: map[string]string{
				"elem1_bar": "bar_val",
				"elem1_foo": "foo_val",
			},
		},
		{
			name: "path_with_multiple_keys_and_target",
			args: args{p: &gnmi.Path{
				Target: "target1",
				Elem: []*gnmi.PathElem{
					{
						Name: "elem1",
						Key: map[string]string{
							"bar": "bar_val",
							"foo": "foo_val",
						},
					},
					{
						Name: "elem2",
					},
				},
			}},
			want: "/elem1/elem2",
			want1: map[string]string{
				"elem1_bar": "bar_val",
				"elem1_foo": "foo_val",
				"target":    "target1",
			},
		},
		{
			name: "path_with_multiple_keys_target_and_origin",
			args: args{p: &gnmi.Path{
				Origin: "origin1",
				Target: "target1",
				Elem: []*gnmi.PathElem{
					{
						Name: "elem1",
						Key: map[string]string{
							"bar": "bar_val",
							"foo": "foo_val",
						},
					},
					{
						Name: "elem2",
					},
				},
			}},
			want: "origin1:/elem1/elem2",
			want1: map[string]string{
				"elem1_bar": "bar_val",
				"elem1_foo": "foo_val",
				"target":    "target1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := TagsFromGNMIPath(tt.args.p)
			if got != tt.want {
				t.Errorf("TagsFromGNMIPath() got = %v, want %v", got, tt.want)
			}
			if !cmp.Equal(got1, tt.want1) {
				t.Errorf("TagsFromGNMIPath() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_getValueFlat(t *testing.T) {
	type args struct {
		prefix   string
		updValue *gnmi.TypedValue
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "simple_json_value",
			args: args{
				prefix: "/configure/router/interface",
				updValue: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_JsonVal{
						JsonVal: []byte(`{
							"admin-state": "enable",
							"ipv4": {
								"primary": {
									"address": "1.1.1.1",
									"prefix-length": 32
								}
							}
						}`),
					},
				},
			},
			want: map[string]interface{}{
				"/configure/router/interface/admin-state":                "enable",
				"/configure/router/interface/ipv4/primary/address":       "1.1.1.1",
				"/configure/router/interface/ipv4/primary/prefix-length": float64(32),
			},
			wantErr: false,
		},
		{
			name: "json_value_with_list",
			args: args{
				prefix: "/network-instance",
				updValue: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_JsonVal{
						JsonVal: []byte(`{
							"interface": [
								"ethernet-1/1",
								"ethernet-1/2",
								"ethernet-1/3",
								"ethernet-1/4"
							]
						}`),
					},
				},
			},
			want: map[string]interface{}{
				"/network-instance/interface.0": "ethernet-1/1",
				"/network-instance/interface.1": "ethernet-1/2",
				"/network-instance/interface.2": "ethernet-1/3",
				"/network-instance/interface.3": "ethernet-1/4",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getValueFlat(tt.args.prefix, tt.args.updValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("getValueFlat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				for k, v := range got {
					fmt.Printf("%s: %v: %T\n", k, v, v)
				}
				t.Errorf("got:  %+v", got)
				t.Errorf("want: %+v", tt.want)
				t.Errorf("getValueFlat() = %v, want %v", got, tt.want)
			}
		})
	}
}
