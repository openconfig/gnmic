// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/AlekSi/pointer"
	"gopkg.in/yaml.v2"

	"github.com/openconfig/gnmic/pkg/api/types"
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
				BufferSize:   uint(100),
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
				BufferSize:   uint(100),
			},
		},
		outErr: nil,
	},
	"from_both_targets_and_main_section": {
		in: []byte(`
metadata:
  key1: val1
  key2: val2
username: admin
password: admin
skip-verify: true
targets:
  10.1.1.1:57400:  
    metadata:
      override1: val2
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
				BufferSize:   uint(100),
				Metadata: map[string]string{
					"override1": "val2",
				},
			},
		},
		outErr: nil,
	},
	"multiple_targets": {
		in: []byte(`
metadata:
  key1: val1
  key2: val2
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
				BufferSize:   uint(100),
				Metadata: map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
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
				BufferSize:   uint(100),
				Metadata: map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
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
				BufferSize:   uint(100),
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
				BufferSize:   uint(100),
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
				BufferSize:   uint(100),
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
				BufferSize:   uint(100),
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
				BufferSize:   uint(100),
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
				BufferSize:   uint(100),
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

var setTargetLoaderConfigDefaultsTest = map[string]struct {
	envs   []string
	in     []byte
	out    *types.TargetConfig
	outErr error
}{
	"from_address": {
		envs: []string{
			"username=user1",
			"pass=pass1",
		},
		in: []byte(`
test1:
    name: test1.123
    address: test1.123:9339
    username: ${username}
    password: ${pass}
    subscriptions:
        - drivenets-sample
`),
		out: &types.TargetConfig{
			Address:       "test1.123:9339",
			Name:          "test1.123",
			Password:      pointer.ToString("pass1"),
			Username:      pointer.ToString("user1"),
			Token:         pointer.ToString(""),
			TLSCert:       pointer.ToString(""),
			TLSKey:        pointer.ToString(""),
			LogTLSSecret:  pointer.ToBool(false),
			Insecure:      pointer.ToBool(false),
			SkipVerify:    pointer.ToBool(false),
			Gzip:          pointer.ToBool(false),
			BufferSize:    uint(100),
			Subscriptions: []string{"drivenets-sample"},
		},
		outErr: nil,
	},
}

func TestSetTargetLoaderConfigDefaults(t *testing.T) {
	for name, data := range setTargetLoaderConfigDefaultsTest {
		t.Run(name, func(t *testing.T) {
			for _, e := range data.envs {
				p := strings.SplitN(e, "=", 2)
				os.Setenv(p[0], p[1])
			}
			var inputMap map[string]*types.TargetConfig
			err := yaml.Unmarshal(data.in, &inputMap)
			if err != nil {
				t.Logf("failed to unmarshal input: %v", err)
				t.Fail()
			}
			var input *types.TargetConfig
			for _, v := range inputMap {
				input = v
				break
			}
			cfg := New()
			err = cfg.SetTargetLoaderConfigDefaults(input)
			if err != nil {
				t.Logf("SetTargetLoaderConfigDefaults error: %v", err)
				t.Fail()
			}
			if !reflect.DeepEqual(input, data.out) {
				t.Logf("expected: %+v", data.out)
				t.Logf("got: %+v", input)
				t.Fail()
			}
		})
	}
}
