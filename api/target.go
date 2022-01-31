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

func Name(name string) TargetOption {
	return func(t *target.Target) error {
		t.Config.Name = name
		return nil
	}
}

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

func Username(username string) TargetOption {
	return func(t *target.Target) error {
		t.Config.Username = pointer.ToString(username)
		return nil
	}
}

func Password(password string) TargetOption {
	return func(t *target.Target) error {
		t.Config.Password = pointer.ToString(password)
		return nil
	}
}

func Timeout(timeout time.Duration) TargetOption {
	return func(t *target.Target) error {
		t.Config.Timeout = timeout
		return nil
	}
}

func Insecure(i bool) TargetOption {
	return func(t *target.Target) error {
		t.Config.Insecure = pointer.ToBool(i)
		return nil
	}
}

func SkipVerify(i bool) TargetOption {
	return func(t *target.Target) error {
		t.Config.SkipVerify = pointer.ToBool(i)
		return nil
	}
}

func TLSCA(tlsca string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSCA = pointer.ToString(tlsca)
		return nil
	}
}

func TLSCert(cert string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSCert = pointer.ToString(cert)
		return nil
	}
}

func TLSKey(key string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSKey = pointer.ToString(key)
		return nil
	}
}

func TLSMinVersion(v string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSMinVersion = v
		return nil
	}
}

func TLSMaxVersion(v string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSMaxVersion = v
		return nil
	}
}

func TLSVersion(v string) TargetOption {
	return func(t *target.Target) error {
		t.Config.TLSVersion = v
		return nil
	}
}

func LogTLSSecret(b bool) TargetOption {
	return func(t *target.Target) error {
		t.Config.LogTLSSecret = pointer.ToBool(b)
		return nil
	}
}

func Gzip(b bool) TargetOption {
	return func(t *target.Target) error {
		t.Config.Gzip = pointer.ToBool(b)
		return nil
	}
}

func Token(token string) TargetOption {
	return func(t *target.Target) error {
		t.Config.Token = pointer.ToString(token)
		return nil
	}
}
