package collector

import (
	"context"
	"errors"
	"fmt"

	"github.com/karimra/gnmic/lockers"
)

func WithLocker(lockerConfig map[string]interface{}) CollectorOption {
	return func(c *Collector) {
		c.lockerConfig = lockerConfig
	}
}

func (c *Collector) InitLocker(ctx context.Context) error {
	if c.lockerConfig == nil {
		return nil
	}
	if lockerType, ok := c.lockerConfig["type"]; ok {
		c.logger.Printf("starting locker type %q", lockerType)
		if initializer, ok := lockers.Lockers[lockerType.(string)]; ok {
			lock := initializer()
			err := lock.Init(ctx, c.lockerConfig, lockers.WithLogger(c.logger))
			if err != nil {
				return err
			}
			c.locker = lock
			return nil
		}
		return fmt.Errorf("unknown locker type %q", lockerType)
	}
	return errors.New("missing locker type field")
}
