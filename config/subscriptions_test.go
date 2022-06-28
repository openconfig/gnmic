package config

import (
	"bytes"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/karimra/gnmic/testutils"
	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/spf13/viper"
)

var getSubscriptionsTestSet = map[string]struct {
	envs   []string
	in     []byte
	out    map[string]*types.SubscriptionConfig
	outErr error
}{
	"no_globals": {
		in: []byte(`
subscriptions:
  sub1:
    paths: 
      - /valid/path
`),
		out: map[string]*types.SubscriptionConfig{
			"sub1": {
				Name:  "sub1",
				Paths: []string{"/valid/path"},
			},
		},
		outErr: nil,
	},
	"with_globals": {
		in: []byte(`
encoding: proto
subscriptions:
  sub1:
    paths: 
      - /valid/path
`),
		out: map[string]*types.SubscriptionConfig{
			"sub1": {
				Name:     "sub1",
				Paths:    []string{"/valid/path"},
				Encoding: "proto",
			},
		},
		outErr: nil,
	},
	"2_subs": {
		in: []byte(`
subscriptions:
  sub1:
    paths: 
      - /valid/path
  sub2:
    paths: 
      - /valid/path2
    mode: stream
    stream-mode: on_change
`),
		out: map[string]*types.SubscriptionConfig{
			"sub1": {
				Name:  "sub1",
				Paths: []string{"/valid/path"},
			},
			"sub2": {
				Name:       "sub2",
				Paths:      []string{"/valid/path2"},
				Mode:       "stream",
				StreamMode: "on_change",
			},
		},
		outErr: nil,
	},
	"2_subs_with_globals": {
		in: []byte(`
encoding: proto
subscriptions:
  sub1:
    paths: 
      - /valid/path
  sub2:
    paths: 
      - /valid/path2
    mode: stream
    stream-mode: on_change
`),
		out: map[string]*types.SubscriptionConfig{
			"sub1": {
				Name:     "sub1",
				Paths:    []string{"/valid/path"},
				Encoding: "proto",
			},
			"sub2": {
				Name:       "sub2",
				Paths:      []string{"/valid/path2"},
				Mode:       "stream",
				StreamMode: "on_change",
				Encoding:   "proto",
			},
		},
		outErr: nil,
	},
	"3_subs_with_env": {
		envs: []string{
			"SUB1_PATH=/valid/path",
			"SUB2_PATH=/valid/path2",
		},
		in: []byte(`
encoding: proto
subscriptions:
  sub1:
    paths: 
      - ${SUB1_PATH}
  sub2:
    paths: 
      - ${SUB2_PATH}
    mode: stream
    stream-mode: on_change
`),
		out: map[string]*types.SubscriptionConfig{
			"sub1": {
				Name:     "sub1",
				Paths:    []string{"/valid/path"},
				Encoding: "proto",
			},
			"sub2": {
				Name:       "sub2",
				Paths:      []string{"/valid/path2"},
				Mode:       "stream",
				StreamMode: "on_change",
				Encoding:   "proto",
			},
		},
		outErr: nil,
	},
	"history_snapshot": {
		in: []byte(`
subscriptions:
  sub1:
    paths: 
      - /valid/path
    history:
      snapshot: 2022-07-14T07:30:00.0Z
`),
		out: map[string]*types.SubscriptionConfig{
			"sub1": {
				Name:  "sub1",
				Paths: []string{"/valid/path"},
				History: &types.HistoryConfig{
					Snapshot: "2022-07-14T07:30:00.0Z",
				},
			},
		},
		outErr: nil,
	},
	"history_range": {
		in: []byte(`
subscriptions:
  sub1:
    paths: 
      - /valid/path
    history:
      start: 2021-07-14T07:30:00.0Z
      end: 2022-07-14T07:30:00.0Z
`),
		out: map[string]*types.SubscriptionConfig{
			"sub1": {
				Name:  "sub1",
				Paths: []string{"/valid/path"},
				History: &types.HistoryConfig{
					Start: "2021-07-14T07:30:00.0Z",
					End:   "2022-07-14T07:30:00.0Z",
				},
			},
		},
		outErr: nil,
	},
}

func TestGetSubscriptions(t *testing.T) {
	for name, data := range getSubscriptionsTestSet {
		t.Run(name, func(t *testing.T) {
			for _, e := range data.envs {
				p := strings.SplitN(e, "=", 2)
				os.Setenv(p[0], p[1])
			}
			cfg := New()
			cfg.Debug = true
			cfg.SetLogger()
			cfg.FileConfig.SetConfigType("yaml")
			err := cfg.FileConfig.ReadConfig(bytes.NewBuffer(data.in))
			if err != nil {
				t.Logf("failed reading config: %v", err)
				t.Fail()
			}
			err = cfg.FileConfig.Unmarshal(cfg)
			if err != nil {
				t.Logf("failed fileConfig.Unmarshal: %v", err)
				t.Fail()
			}
			v := cfg.FileConfig.Get("subscriptions")
			t.Logf("raw interface subscriptions: %+v", v)
			outs, err := cfg.GetSubscriptions(nil)
			t.Logf("exp value: %+v", data.out)
			t.Logf("got value: %+v", outs)
			if err != nil {
				t.Logf("failed getting subscriptions: %v", err)
				t.Fail()
			}
			if !reflect.DeepEqual(outs, data.out) {
				t.Log("maps not equal")
				t.Fail()
			}
		})
	}
}

func TestConfig_CreateSubscribeRequest(t *testing.T) {
	type fields struct {
		GlobalFlags        GlobalFlags
		LocalFlags         LocalFlags
		FileConfig         *viper.Viper
		Targets            map[string]*types.TargetConfig
		Subscriptions      map[string]*types.SubscriptionConfig
		Outputs            map[string]map[string]interface{}
		Inputs             map[string]map[string]interface{}
		Processors         map[string]map[string]interface{}
		Clustering         *clustering
		GnmiServer         *gnmiServer
		APIServer          *APIServer
		Loader             map[string]interface{}
		Actions            map[string]map[string]interface{}
		logger             *log.Logger
		setRequestTemplate []*template.Template
		setRequestVars     map[string]interface{}
	}
	type args struct {
		sc     *types.SubscriptionConfig
		target string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *gnmi.SubscribeRequest
		wantErr bool
	}{
		{
			name: "once_subscription",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
					},
					Mode:     "once",
					Encoding: "json_ietf",
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Subscription: []*gnmi.Subscription{
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
						},
						Mode:     gnmi.SubscriptionList_ONCE,
						Encoding: gnmi.Encoding_JSON_IETF,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "once_subscription_multiple_paths",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
						"network-instance",
					},
					Mode:     "once",
					Encoding: "json_ietf",
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Mode: gnmi.SubscriptionList_ONCE,
						Subscription: []*gnmi.Subscription{
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "network-instance",
									}},
								},
							},
						},
						Encoding: gnmi.Encoding_JSON_IETF,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "poll_subscription",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
					},
					Mode:     "poll",
					Encoding: "json_ietf",
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Subscription: []*gnmi.Subscription{
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
						},
						Mode:     gnmi.SubscriptionList_POLL,
						Encoding: gnmi.Encoding_JSON_IETF,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "poll_subscription",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
						"network-instance",
					},
					Mode:     "poll",
					Encoding: "json_ietf",
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Subscription: []*gnmi.Subscription{
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "network-instance",
									}},
								},
							},
						},
						Mode:     gnmi.SubscriptionList_POLL,
						Encoding: gnmi.Encoding_JSON_IETF,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "stream_subscription",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
					},
					Mode:     "stream",
					Encoding: "json_ietf",
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Subscription: []*gnmi.Subscription{
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
						},
						Mode:     gnmi.SubscriptionList_STREAM,
						Encoding: gnmi.Encoding_JSON_IETF,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "stream_subscription_multiple_paths",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
						"network-instance",
					},
					Mode:     "stream",
					Encoding: "json_ietf",
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Subscription: []*gnmi.Subscription{
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "network-instance",
									}},
								},
							},
						},
						Mode:     gnmi.SubscriptionList_STREAM,
						Encoding: gnmi.Encoding_JSON_IETF,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "stream_sample_subscription",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
					},
					// Mode:       "stream",
					StreamMode: "sample",
					Encoding:   "json_ietf",
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Subscription: []*gnmi.Subscription{
							{
								Mode: gnmi.SubscriptionMode_SAMPLE,
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
						},
						Encoding: gnmi.Encoding_JSON_IETF,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "stream_on_change_subscription",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
					},
					// Mode:       "stream",
					StreamMode: "on-change",
					Encoding:   "json_ietf",
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Subscription: []*gnmi.Subscription{
							{
								Mode: gnmi.SubscriptionMode_ON_CHANGE,
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
						},
						Encoding: gnmi.Encoding_JSON_IETF,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "stream_target_defined_subscription",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
					},
					// Mode:       "stream",
					StreamMode: "on-change",
					Encoding:   "json_ietf",
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Subscription: []*gnmi.Subscription{
							{
								Mode: gnmi.SubscriptionMode_TARGET_DEFINED,
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
						},
						Encoding: gnmi.Encoding_JSON_IETF,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "subscription_with_history_snapshot",
			args: args{
				sc: &types.SubscriptionConfig{
					Paths: []string{
						"interface",
					},
					Mode:     "once",
					Encoding: "json_ietf",
					History: &types.HistoryConfig{
						Snapshot: "2022-07-14T07:30:00.0Z",
					},
				},
			},
			want: &gnmi.SubscribeRequest{
				Request: &gnmi.SubscribeRequest_Subscribe{
					Subscribe: &gnmi.SubscriptionList{
						Subscription: []*gnmi.Subscription{
							{
								Path: &gnmi.Path{
									Elem: []*gnmi.PathElem{{
										Name: "interface",
									}},
								},
							},
						},
						Encoding: gnmi.Encoding_JSON_IETF,
						Mode:     gnmi.SubscriptionList_ONCE,
					},
				},
				Extension: []*gnmi_ext.Extension{
					{
						Ext: &gnmi_ext.Extension_History{
							History: &gnmi_ext.History{
								Request: &gnmi_ext.History_SnapshotTime{
									SnapshotTime: 1657783800000000,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				GlobalFlags:        tt.fields.GlobalFlags,
				LocalFlags:         tt.fields.LocalFlags,
				FileConfig:         tt.fields.FileConfig,
				Targets:            tt.fields.Targets,
				Subscriptions:      tt.fields.Subscriptions,
				Outputs:            tt.fields.Outputs,
				Inputs:             tt.fields.Inputs,
				Processors:         tt.fields.Processors,
				Clustering:         tt.fields.Clustering,
				GnmiServer:         tt.fields.GnmiServer,
				APIServer:          tt.fields.APIServer,
				Loader:             tt.fields.Loader,
				Actions:            tt.fields.Actions,
				logger:             tt.fields.logger,
				setRequestTemplate: tt.fields.setRequestTemplate,
				setRequestVars:     tt.fields.setRequestVars,
			}
			got, err := c.CreateSubscribeRequest(tt.args.sc, tt.args.target)
			if (err != nil) != tt.wantErr {
				t.Logf("Config.CreateSubscribeRequest() error   = %v", err)
				t.Logf("Config.CreateSubscribeRequest() wantErr = %v", tt.wantErr)
				t.Fail()
				return
			}
			if !testutils.SubscribeRequestsEqual(got, tt.want) {
				t.Logf("Config.CreateSubscribeRequest() got  = %v", got)
				t.Logf("Config.CreateSubscribeRequest() want = %v", tt.want)
				t.Fail()
			}
		})
	}
}
