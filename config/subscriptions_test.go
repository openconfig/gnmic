package config

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/karimra/gnmic/types"
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
