package lockers

import (
	"context"
	"errors"
	"log"

	"github.com/mitchellh/mapstructure"
)

var (
	ErrCanceled = errors.New("canceled")
)

type Locker interface {
	Init(context.Context, map[string]interface{}, ...Option) error
	Lock(context.Context, string, []byte) (bool, error)
	KeepLock(context.Context, string) (chan struct{}, chan error)
	Unlock(string) error
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
