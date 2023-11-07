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

var ErrCanceled = errors.New("canceled")

type Locker interface {
	// Init initialises the locker data, with the given configuration read from flags/files.
	Init(context.Context, map[string]interface{}, ...Option) error
	// Stop is called when the locker instance is called. It should unlock all aquired locks.
	Stop() error
	SetLogger(*log.Logger)

	// This is the Target locking logic.

	// Lock acquires a lock on given key.
	Lock(context.Context, string, []byte) (bool, error)
	// KeepLock maintains the lock on the target.
	KeepLock(context.Context, string) (chan struct{}, chan error)
	// IsLocked replys if the target given as string is currently locked or not.
	IsLocked(context.Context, string) (bool, error)
	// Unlock unlocks the target log.
	Unlock(context.Context, string) error

	// This is the instance registration logic.

	// Register registers this instance in the registry. It must also maintain the registration (called in a goroutine from the main). ServiceRegistration.ID contains the ID of the service to register.
	Register(context.Context, *ServiceRegistration) error
	// Deregister removes this instance from the registry. This looks like it's not called.
	Deregister(string) error

	// GetServices must return the gnmic instances.
	GetServices(ctx context.Context, serviceName string, tags []string) ([]*Service, error)
	// WatchServices must push all existing discovered gnmic instances
	// into the provided channel.
	WatchServices(ctx context.Context, serviceName string, tags []string, ch chan<- []*Service, dur time.Duration) error

	// Mixed registration/target lock functions

	// List returns all locks that start with prefix string,
	// indexed by the lock name. Could be target locks or leader lock. It must return a map of matching keys to instance name.
	List(ctx context.Context, prefix string) (map[string]string, error)
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
	"redis",
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
