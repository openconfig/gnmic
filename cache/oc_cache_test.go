// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
)

func Test_gnmiCache_read(t *testing.T) {
	type input struct {
		measName string
		target   string
		m        *gnmi.SubscribeResponse
	}
	type fields struct {
		inputs []input
	}
	type args struct {
		sub    string
		target string
		p      *gnmi.Path
	}
	tests := []struct {
		name              string
		fields            fields
		args              args
		want              map[string][]*gnmi.Notification
		expectedRespCount int
	}{
		{
			name: "test1",
			fields: fields{
				inputs: []input{
					{
						measName: "sub1",
						target:   "t1",
						m: &gnmi.SubscribeResponse{
							Response: &gnmi.SubscribeResponse_Update{
								Update: &gnmi.Notification{
									Timestamp: time.Now().UnixNano(),
									Prefix:    &gnmi.Path{Target: "t1"},
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
												Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "srl1"},
											},
										},
									},
								},
							},
						},
					},
					{
						measName: "sub1",
						target:   "t1",
						m: &gnmi.SubscribeResponse{
							Response: &gnmi.SubscribeResponse_Update{
								Update: &gnmi.Notification{
									Timestamp: time.Now().UnixNano(),
									Prefix:    &gnmi.Path{Target: "t1"},
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
													{Name: "admin-state"},
												},
											},
											Val: &gnmi.TypedValue{
												Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "enable"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				sub:    "sub1",
				target: "*",
				p: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "system"},
						{Name: "name"},
						{Name: "host-name"},
					}},
			},
			want:              map[string][]*gnmi.Notification{},
			expectedRespCount: 1,
		},
		{
			name: "test2",
			fields: fields{
				inputs: []input{
					{
						measName: "sub1",
						target:   "t1",
						m: &gnmi.SubscribeResponse{
							Response: &gnmi.SubscribeResponse_Update{
								Update: &gnmi.Notification{
									Timestamp: time.Now().UnixNano(),
									Prefix:    &gnmi.Path{Target: "t1"},
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
												Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "srl1"},
											},
										},
									},
								},
							},
						},
					},
					{
						measName: "sub1",
						target:   "t1",
						m: &gnmi.SubscribeResponse{
							Response: &gnmi.SubscribeResponse_Update{
								Update: &gnmi.Notification{
									Timestamp: time.Now().UnixNano(),
									Prefix:    &gnmi.Path{Target: "t1"},
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
													{Name: "admin-state"},
												},
											},
											Val: &gnmi.TypedValue{
												Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "enable"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				sub:    "sub1",
				target: "*",
				p: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{
							Name: "interface",
							Key: map[string]string{
								"name": "ethernet-1/1",
							},
						},
						{Name: "admin-state"},
					}},
			},
			want:              map[string][]*gnmi.Notification{},
			expectedRespCount: 1,
		},
		{
			name: "readAll_same_subscription",
			fields: fields{
				inputs: []input{
					{
						measName: "sub1",
						target:   "t1",
						m: &gnmi.SubscribeResponse{
							Response: &gnmi.SubscribeResponse_Update{
								Update: &gnmi.Notification{
									Timestamp: time.Now().UnixNano(),
									Prefix:    &gnmi.Path{Target: "t1"},
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
												Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "srl1"},
											},
										},
									},
								},
							},
						},
					},
					{
						measName: "sub1",
						target:   "t1",
						m: &gnmi.SubscribeResponse{
							Response: &gnmi.SubscribeResponse_Update{
								Update: &gnmi.Notification{
									Timestamp: time.Now().UnixNano(),
									Prefix:    &gnmi.Path{Target: "t1"},
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
													{Name: "admin-state"},
												},
											},
											Val: &gnmi.TypedValue{
												Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "enable"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				sub:    "",
				target: "*",
				p:      nil,
			},
			want:              map[string][]*gnmi.Notification{},
			expectedRespCount: 2,
		},
		{
			name: "readAll",
			fields: fields{
				inputs: []input{
					{
						measName: "sub1",
						target:   "t1",
						m: &gnmi.SubscribeResponse{
							Response: &gnmi.SubscribeResponse_Update{
								Update: &gnmi.Notification{
									Timestamp: time.Now().UnixNano(),
									Prefix:    &gnmi.Path{Target: "t1"},
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
												Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "srl1"},
											},
										},
									},
								},
							},
						},
					},
					{
						measName: "sub2",
						target:   "t1",
						m: &gnmi.SubscribeResponse{
							Response: &gnmi.SubscribeResponse_Update{
								Update: &gnmi.Notification{
									Timestamp: time.Now().UnixNano(),
									Prefix:    &gnmi.Path{Target: "t1"},
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
													{Name: "admin-state"},
												},
											},
											Val: &gnmi.TypedValue{
												Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "enable"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				sub:    "",
				target: "*",
				p:      nil,
			},
			want:              map[string][]*gnmi.Notification{},
			expectedRespCount: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gc := newGNMICache(&Config{}, "oc", WithLogger(log.Default()))
			for _, in := range tt.fields.inputs {
				gc.Write(context.TODO(), in.measName, in.m)
			}

			rsp := gc.read(tt.args.sub, tt.args.target, tt.args.p)
			if _, ok := rsp[tt.args.sub]; !ok && tt.args.sub != "" {
				t.Errorf("%s: response does not contain the expected subscription name", tt.name)
			}
			var rspCount int
			if tt.args.sub == "" {
				for _, rsps := range rsp {
					rspCount += len(rsps)
				}
			} else {
				rspCount = len(rsp[tt.args.sub])
			}
			if tt.expectedRespCount != rspCount {
				t.Errorf("%s: unexpected response count, got %d, expected %d", tt.name, rspCount, tt.expectedRespCount)
			}

		})
	}
}
