package config

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/karimra/gnmic/collector"
)

var getTargetsTestSet = map[string]struct {
	in     []byte
	out    map[string]*collector.TargetConfig
	outErr error
}{
	"from_address": {
		in: []byte(`
port: 57400
username: admin
password: admin
address: 10.1.1.1
`),
		out: map[string]*collector.TargetConfig{
			"10.1.1.1:57400": {
				Address:    "10.1.1.1:57400",
				Name:       "10.1.1.1:57400",
				Password:   &adminStr,
				Username:   &adminStr,
				TLSCert:    &emptyStr,
				TLSKey:     &emptyStr,
				Insecure:   &falseBool,
				SkipVerify: &falseBool,
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
		out: map[string]*collector.TargetConfig{
			"10.1.1.1:57400": {
				Address:    "10.1.1.1:57400",
				Name:       "10.1.1.1:57400",
				Password:   &adminStr,
				Username:   &adminStr,
				TLSCert:    &emptyStr,
				TLSKey:     &emptyStr,
				Insecure:   &falseBool,
				SkipVerify: &falseBool,
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
		out: map[string]*collector.TargetConfig{
			"10.1.1.1:57400": {
				Address:    "10.1.1.1:57400",
				Name:       "10.1.1.1:57400",
				Password:   &adminStr,
				Username:   &adminStr,
				TLSCert:    &emptyStr,
				TLSKey:     &emptyStr,
				Insecure:   &falseBool,
				SkipVerify: &trueBool,
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
		out: map[string]*collector.TargetConfig{
			"10.1.1.1:57400": {
				Address:    "10.1.1.1:57400",
				Name:       "10.1.1.1:57400",
				Password:   &adminStr,
				Username:   &adminStr,
				TLSCert:    &emptyStr,
				TLSKey:     &emptyStr,
				Insecure:   &falseBool,
				SkipVerify: &falseBool,
			},
			"10.1.1.2:57400": {
				Address:    "10.1.1.2:57400",
				Name:       "10.1.1.2:57400",
				Password:   &adminStr,
				Username:   &adminStr,
				TLSCert:    &emptyStr,
				TLSKey:     &emptyStr,
				Insecure:   &falseBool,
				SkipVerify: &falseBool,
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
		out: map[string]*collector.TargetConfig{
			"10.1.1.1:57400": {
				Address:    "10.1.1.1:57400",
				Name:       "10.1.1.1:57400",
				Password:   &adminStr,
				Username:   &adminStr,
				TLSCert:    &emptyStr,
				TLSKey:     &emptyStr,
				Insecure:   &falseBool,
				SkipVerify: &trueBool,
			},
			"10.1.1.2:57400": {
				Address:    "10.1.1.2:57400",
				Name:       "10.1.1.2:57400",
				Password:   &adminStr,
				Username:   &adminStr,
				TLSCert:    &emptyStr,
				TLSKey:     &emptyStr,
				Insecure:   &falseBool,
				SkipVerify: &trueBool,
			},
		},
		outErr: nil,
	},
}

func TestGetTargets(t *testing.T) {
	for name, data := range getTargetsTestSet {
		t.Run(name, func(t *testing.T) {
			cfg := New()
			cfg.Globals.Debug = true
			cfg.SetLogger()
			cfg.FileConfig.SetConfigType("yaml")
			err := cfg.FileConfig.ReadConfig(bytes.NewBuffer(data.in))
			if err != nil {
				t.Logf("failed reading config: %v", err)
				t.Fail()
			}
			err = cfg.FileConfig.Unmarshal(cfg.Globals)
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
