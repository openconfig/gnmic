package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/karimra/gnmic/lockers"
)

func (a *App) InitLocker(ctx context.Context, lockerCfg map[string]interface{}) error {
	if lockerCfg == nil {
		return nil
	}
	if lockerType, ok := lockerCfg["type"]; ok {
		a.Logger.Printf("starting locker type %q", lockerType)
		if initializer, ok := lockers.Lockers[lockerType.(string)]; ok {
			lock := initializer()
			err := lock.Init(ctx, lockerCfg, lockers.WithLogger(a.Logger))
			if err != nil {
				return err
			}
			a.locker = lock
			return nil
		}
		return fmt.Errorf("unknown locker type %q", lockerType)
	}
	return errors.New("missing locker type field")
}
