package lockers

import (
	"context"

	"github.com/mitchellh/mapstructure"
)

type Locker interface {
	Init(context.Context, map[string]interface{}) error
	Lock(context.Context, string) (bool, error)
	LockMany(context.Context, []string, chan string)
	Unlock(string) error
	Stop() error
}

type Initializer func() Locker

var Lockers = map[string]Initializer{}

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
