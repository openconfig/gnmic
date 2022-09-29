// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package lockers

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/mitchellh/mapstructure"
)

var (
	ErrCanceled = errors.New("canceled")
)

type Locker interface {
	Init(context.Context, map[string]interface{}, ...Option) error

	Lock(context.Context, string, []byte) (bool, error)
	KeepLock(context.Context, string) (chan struct{}, chan error)
	IsLocked(context.Context, string) (bool, error)
	Unlock(context.Context, string) error

	Register(context.Context, *ServiceRegistration) error
	Deregister(string) error

	List(context.Context, string) (map[string]string, error)
	GetServices(ctx context.Context, serviceName string, tags []string) ([]*Service, error)
	WatchServices(ctx context.Context, serviceName string, tags []string, ch chan<- []*Service, dur time.Duration) error

	Stop() error
	SetLogger(*log.Logger)
}

type Initializer func() Locker

var Lockers = map[string]Initializer{}

type Option func(Locker)

func WithLogger(logger *log.Logger) Option {
	return func(i Locker) {
		i.SetLogger(logger)
	}
}

var LockerTypes = []string{
	"consul",
	"k8s",
}

func Register(name string, initFn Initializer) {
	Lockers[name] = initFn
}

func DecodeConfig(src, dst interface{}) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     dst,
		},
	)
	if err != nil {
		return err
	}
	return decoder.Decode(src)
}

type ServiceRegistration struct {
	ID      string
	Name    string
	Address string
	Port    int
	Tags    []string
	TTL     time.Duration
}

type Service struct {
	ID      string
	Address string
	Tags    []string
}
