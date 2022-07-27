package cache

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

func Test_natsCache_Write(t *testing.T) {
	type fields struct {
		cfg *Config
	}
	type args struct {
		ctx              context.Context
		subscriptionName string
		m                proto.Message
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test1",
			fields: fields{
				cfg: &Config{
					Type: cacheType_JS,
				},
			},
			args: args{
				ctx:              context.TODO(),
				subscriptionName: "sub1",
				m: &gnmi.SubscribeResponse{
					Response: &gnmi.SubscribeResponse_Update{
						Update: &gnmi.Notification{
							Prefix: &gnmi.Path{
								Target: "router1",
							},
							Timestamp: time.Now().UnixNano(),
							Update: []*gnmi.Update{
								{
									Path: &gnmi.Path{
										Elem: []*gnmi.PathElem{
											{
												Name: "interface",
												Key: map[string]string{
													"name": "ethernet-1/1",
												},
											},
											{
												Name: "description",
											},
										},
									},
									Val: &gnmi.TypedValue{
										Value: &gnmi.TypedValue_AsciiVal{
											AsciiVal: "interface_description",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "test2",
			fields: fields{
				cfg: &Config{
					Type: cacheType_JS,
				},
			},
			args: args{
				ctx:              context.TODO(),
				subscriptionName: "sub1",
				m: &gnmi.SubscribeResponse{
					Response: &gnmi.SubscribeResponse_Update{
						Update: &gnmi.Notification{
							Prefix: &gnmi.Path{
								Target: "router1",
							},
							Timestamp: time.Now().UnixNano(),
							Update: []*gnmi.Update{
								{
									Path: &gnmi.Path{
										Elem: []*gnmi.PathElem{
											{
												Name: "interface",
												Key: map[string]string{
													"name": "ethernet-1/1",
												},
											},
											{
												Name: "description",
											},
										},
									},
									Val: &gnmi.TypedValue{
										Value: &gnmi.TypedValue_AsciiVal{
											AsciiVal: "interface_description",
										},
									},
								},
								{
									Path: &gnmi.Path{
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
											{
												Name: "in-octets",
											},
										},
									},
									Val: &gnmi.TypedValue{
										Value: &gnmi.TypedValue_AsciiVal{
											AsciiVal: "42",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(tt.fields.cfg, WithLogger(log.Default()))
			if err != nil {
				t.Fatal(err)
			}
			c.Write(tt.args.ctx, tt.args.subscriptionName, tt.args.m)
			rs, err := c.Read(context.TODO(), "reader1", nil)
			if err != nil {
				t.Fatal(err)
			}
			for s, ns := range rs {
				t.Logf("sub %s, read %d msgs: %+v", s, len(ns), ns)
			}
		})
	}
}
