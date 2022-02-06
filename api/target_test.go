// Copyright © 2022 Karim Radhouani <medkarimrdi@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/google/go-cmp/cmp"
	"github.com/karimra/gnmic/types"
)

type input struct {
	opts   []TargetOption
	config *types.TargetConfig
}

var targetTestSet = map[string]input{
	"address": {
		opts: []TargetOption{
			Address("10.0.0.1:57400"),
			Insecure(true),
		},
		config: &types.TargetConfig{
			Name:       "10.0.0.1:57400",
			Address:    "10.0.0.1:57400",
			Insecure:   pointer.ToBool(true),
			SkipVerify: pointer.ToBool(false),
			Timeout:    DefaultTargetTimeout,
		},
	},
	"username": {
		opts: []TargetOption{
			Address("10.0.0.1:57400"),
			Username("admin"),
		},
		config: &types.TargetConfig{
			Name:       "10.0.0.1:57400",
			Address:    "10.0.0.1:57400",
			Username:   pointer.ToString("admin"),
			Insecure:   pointer.ToBool(false),
			SkipVerify: pointer.ToBool(false),
			Timeout:    DefaultTargetTimeout,
		},
	},
	"two_addresses": {
		opts: []TargetOption{
			Address("10.0.0.1:57400"),
			Address("10.0.0.2:57400"),
			Insecure(true),
		},
		config: &types.TargetConfig{
			Name:       "10.0.0.1:57400",
			Address:    "10.0.0.1:57400,10.0.0.2:57400",
			Insecure:   pointer.ToBool(true),
			SkipVerify: pointer.ToBool(false),
			Timeout:    DefaultTargetTimeout,
		},
	},
	"skip_verify": {
		opts: []TargetOption{
			Address("10.0.0.1:57400"),
			SkipVerify(true),
		},
		config: &types.TargetConfig{
			Name:       "10.0.0.1:57400",
			Address:    "10.0.0.1:57400",
			Insecure:   pointer.ToBool(false),
			SkipVerify: pointer.ToBool(true),
			Timeout:    DefaultTargetTimeout,
		},
	},
	"tlsca": {
		opts: []TargetOption{
			Address("10.0.0.1:57400"),
			TLSCA("tlsca_path"),
		},
		config: &types.TargetConfig{
			Name:       "10.0.0.1:57400",
			Address:    "10.0.0.1:57400",
			Insecure:   pointer.ToBool(false),
			SkipVerify: pointer.ToBool(false),
			Timeout:    DefaultTargetTimeout,
			TLSCA:      pointer.ToString("tlsca_path"),
		},
	},
	"tls_key_cert": {
		opts: []TargetOption{
			Address("10.0.0.1:57400"),
			TLSKey("tlskey_path"),
			TLSCert("tlscert_path"),
		},
		config: &types.TargetConfig{
			Name:       "10.0.0.1:57400",
			Address:    "10.0.0.1:57400",
			Insecure:   pointer.ToBool(false),
			SkipVerify: pointer.ToBool(false),
			Timeout:    DefaultTargetTimeout,
			TLSKey:     pointer.ToString("tlskey_path"),
			TLSCert:    pointer.ToString("tlscert_path"),
		},
	},
	"token": {
		opts: []TargetOption{
			Address("10.0.0.1:57400"),
			Token("token_value"),
		},
		config: &types.TargetConfig{
			Name:       "10.0.0.1:57400",
			Address:    "10.0.0.1:57400",
			Insecure:   pointer.ToBool(false),
			SkipVerify: pointer.ToBool(false),
			Timeout:    DefaultTargetTimeout,
			Token:      pointer.ToString("token_value"),
		},
	},
	"gzip": {
		opts: []TargetOption{
			Address("10.0.0.1:57400"),
			Gzip(true),
		},
		config: &types.TargetConfig{
			Name:       "10.0.0.1:57400",
			Address:    "10.0.0.1:57400",
			Insecure:   pointer.ToBool(false),
			SkipVerify: pointer.ToBool(false),
			Timeout:    DefaultTargetTimeout,
			Gzip:       pointer.ToBool(true),
		},
	},
}

func TestNewTarget(t *testing.T) {
	for name, item := range targetTestSet {
		t.Run(name, func(t *testing.T) {
			tg, err := NewTarget(item.opts...)
			if err != nil {
				t.Errorf("failed at %q: %v", name, err)
				t.Fail()
			}
			if !cmp.Equal(tg.Config, item.config) {
				t.Errorf("failed at %q", name)
				t.Errorf("expected %+v", item.config)
				t.Errorf("     got %+v", tg.Config)
				t.Fail()
			}
		})
	}
}
