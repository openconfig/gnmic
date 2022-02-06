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
	"errors"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/karimra/gnmic/target"
	"github.com/karimra/gnmic/types"
)

var DefaultTargetTimeout = 10 * time.Second

type TargetOption func(*target.Target) error

func NewTarget(opts ...TargetOption) (*target.Target, error) {
	t := target.NewTarget(&types.TargetConfig{})
	var err error
	for _, o := range opts {
		err = o(t)
		if err != nil {
			return nil, err
		}
	}
	if t.Config.Address == "" {
		return nil, errors.New("missing address")
	}
	if t.Config.Name == "" {
		t.Config.Name = strings.Split(t.Config.Address, ",")[0]
	}
	if t.Config.Timeout == 0 {
		t.Config.Timeout = DefaultTargetTimeout
	}
	if t.Config.Insecure == nil && t.Config.SkipVerify == nil {
		t.Config.Insecure = pointer.ToBool(false)
		t.Config.SkipVerify = pointer.ToBool(false)
	}
	if t.Config.SkipVerify == nil {
		t.Config.SkipVerify = pointer.ToBool(false)
	}
	if t.Config.Insecure == nil {
		t.Config.Insecure = pointer.ToBool(false)
	}
	return t, nil
}

// Name sets the target name.
func Name(name string) TargetOption {
	return func(t *target.Target) error {
		t.Config.Name = name
		return nil
	}
}

// Address sets the target address.
// This Option can be set multiple times.
func Address(addr string) TargetOption {
	return func(t *target.Target) error {
		if t.Config.Address != "" {
			t.Config.Address = strings.Join([]string{t.Config.Address, addr}, ",")
			return nil
		}
		t.Config.Address = addr
		return nil
	}
}

// Username sets the target Username.
func Username(username string) TargetOption {
	return func(t *target.Target) error {
		t.Config.Username = pointer.ToString(username)
		return nil
	}
}

// Password sets the target Password.
func Password(password string) TargetOption {
	return func(t *target.Target) error {
		t.Config.Password = pointer.ToString(password)
		return nil
	}
}

// Timeout sets the gNMI client creation timeout.
func Timeout(timeout time.Duration) TargetOption {
	return func(t *target.Target) error {
		t.Config.Timeout = timeout
		return nil
	}
}

// Insecure sets the option to create a gNMI client with an
// insecure gRPC connection
func Insecure(i bool) TargetOption {
	return func(t *target.Target) error {
		t.Config.Insecure = pointer.ToBool(i)
		return nil
	}
}

// SkipVerify sets the option to create a gNMI client with a
// secure gRPC connection without verifying the target's certificates.
func SkipVerify(i bool) TargetOption {
	return func(t *target.Target) error {
		t.Config.SkipVerify = pointer.ToBool(i)
		return nil
	}
}

// TLSCA sets that path towards the TLS certificate authority file.
func TLSCA(tlsca string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSCA = pointer.ToString(tlsca)
		return nil
	}
}

// TLSCert sets that path towards the TLS certificate file.
func TLSCert(cert string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSCert = pointer.ToString(cert)
		return nil
	}
}

// TLSKey sets that path towards the TLS key file.
func TLSKey(key string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSKey = pointer.ToString(key)
		return nil
	}
}

// TLSMinVersion sets the TLS minimum version used during the TLS handshake.
func TLSMinVersion(v string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSMinVersion = v
		return nil
	}
}

// TLSMaxVersion sets the TLS maximum version used during the TLS handshake.
func TLSMaxVersion(v string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSMaxVersion = v
		return nil
	}
}

// TLSVersion sets the desired TLS version used during the TLS handshake.
func TLSVersion(v string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSVersion = v
		return nil
	}
}

// LogTLSSecret, if set to true,
// enables logging of the TLS master key.
func LogTLSSecret(b bool) TargetOption {
	return func(t *target.Target) error {
		t.Config.LogTLSSecret = pointer.ToBool(b)
		return nil
	}
}

// Gzip, if set to true,
// adds gzip compression to the gRPC connection.
func Gzip(b bool) TargetOption {
	return func(t *target.Target) error {
		t.Config.Gzip = pointer.ToBool(b)
		return nil
	}
}

// Token sets the per RPC credentials for all RPC calls.
func Token(token string) TargetOption {
	return func(t *target.Target) error {
		t.Config.Token = pointer.ToString(token)
		return nil
	}
}
