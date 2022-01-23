package config

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/karimra/gnmic/types"
)

var getTargetsTestSet = map[string]struct {
	envs   []string
	in     []byte
	out    map[string]*types.TargetConfig
	outErr error
}{
	"from_address": {
		in: []byte(`
port: 57400
username: admin
password: admin
address: 10.1.1.1
`),
		out: map[string]*types.TargetConfig{
			"10.1.1.1": {
				Address:      "10.1.1.1:57400",
				Name:         "10.1.1.1",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(false),
				Gzip:         pointer.ToBool(false),
			},
		},
		outErr: nil,
	},
	"from_targets_only": {
		in: []byte(`
targets:
  10.1.1.1:57400:  
    username: admin
    password: admin
`),
		out: map[string]*types.TargetConfig{
			"10.1.1.1:57400": {
				Address:      "10.1.1.1:57400",
				Name:         "10.1.1.1:57400",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(false),
				Gzip:         pointer.ToBool(false),
			},
		},
		outErr: nil,
	},
	"from_both_targets_and_main_section": {
		in: []byte(`
username: admin
password: admin
skip-verify: true
targets:
  10.1.1.1:57400:  
`),
		out: map[string]*types.TargetConfig{
			"10.1.1.1:57400": {
				Address:      "10.1.1.1:57400",
				Name:         "10.1.1.1:57400",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(true),
				Gzip:         pointer.ToBool(false),
			},
		},
		outErr: nil,
	},
	"multiple_targets": {
		in: []byte(`
targets:
  10.1.1.1:57400:
    username: admin
    password: admin
  10.1.1.2:57400:
    username: admin
    password: admin
`),
		out: map[string]*types.TargetConfig{
			"10.1.1.1:57400": {
				Address:      "10.1.1.1:57400",
				Name:         "10.1.1.1:57400",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(false),
				Gzip:         pointer.ToBool(false),
			},
			"10.1.1.2:57400": {
				Address:      "10.1.1.2:57400",
				Name:         "10.1.1.2:57400",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(false),
				Gzip:         pointer.ToBool(false),
			},
		},
		outErr: nil,
	},
	"multiple_targets_from_main_section": {
		in: []byte(`
skip-verify: true
targets:
  10.1.1.1:57400:
    username: admin
    password: admin
  10.1.1.2:57400:
    username: admin
    password: admin
`),
		out: map[string]*types.TargetConfig{
			"10.1.1.1:57400": {
				Address:      "10.1.1.1:57400",
				Name:         "10.1.1.1:57400",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(true),
				Gzip:         pointer.ToBool(false),
			},
			"10.1.1.2:57400": {
				Address:      "10.1.1.2:57400",
				Name:         "10.1.1.2:57400",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(true),
				Gzip:         pointer.ToBool(false),
			},
		},
		outErr: nil,
	},
	"multiple_targets_with_gzip": {
		in: []byte(`
skip-verify: true
targets:
  10.1.1.1:57400:
    username: admin
    password: admin
    gzip: true
  10.1.1.2:57400:
    username: admin
    password: admin
`),
		out: map[string]*types.TargetConfig{
			"10.1.1.1:57400": {
				Address:      "10.1.1.1:57400",
				Name:         "10.1.1.1:57400",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(true),
				Gzip:         pointer.ToBool(true),
			},
			"10.1.1.2:57400": {
				Address:      "10.1.1.2:57400",
				Name:         "10.1.1.2:57400",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(true),
				Gzip:         pointer.ToBool(false),
			},
		},
		outErr: nil,
	},
	"with_envs": {
		envs: []string{
			"SUB_NAME=sub1",
			"OUT_NAME=o1",
		},
		in: []byte(`
skip-verify: true
targets:
  10.1.1.1:57400:
    username: admin
    password: admin
    outputs:
      - ${OUT_NAME}
    subscriptions:
      - ${SUB_NAME}
`),
		out: map[string]*types.TargetConfig{
			"10.1.1.1:57400": {
				Address:      "10.1.1.1:57400",
				Name:         "10.1.1.1:57400",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(true),
				Gzip:         pointer.ToBool(false),
				Subscriptions: []string{
					"sub1",
				},
				Outputs: []string{
					"o1",
				},
			},
		},
		outErr: nil,
	},
	"target_with_multiple_addresses": {
		in: []byte(`
port: 57400
targets:
  target1:
    username: admin
    password: admin
    address: 10.1.1.1,10.1.1.2
`),
		out: map[string]*types.TargetConfig{
			"target1": {
				Address:      "10.1.1.1:57400,10.1.1.2:57400",
				Name:         "target1",
				Password:     pointer.ToString("admin"),
				Username:     pointer.ToString("admin"),
				Token:        pointer.ToString(""),
				TLSCert:      pointer.ToString(""),
				TLSKey:       pointer.ToString(""),
				LogTLSSecret: pointer.ToBool(false),
				Insecure:     pointer.ToBool(false),
				SkipVerify:   pointer.ToBool(false),
				Gzip:         pointer.ToBool(false),
			},
		},
		outErr: nil,
	},
}

func TestGetTargets(t *testing.T) {
	for name, data := range getTargetsTestSet {
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
			v := cfg.FileConfig.Get("targets")
			t.Logf("raw interface targets: %+v", v)
			outs, err := cfg.GetTargets()
			t.Logf("exp value: %+v", data.out)
			t.Logf("got value: %+v", outs)
			if err != nil {
				t.Logf("failed getting targets: %v", err)
				t.Fail()
			}
			if !reflect.DeepEqual(outs, data.out) {
				t.Log("maps not equal")
				t.Fail()
			}
		})
	}
}
