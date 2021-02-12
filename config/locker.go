package config

import (
	"errors"

	"github.com/karimra/gnmic/lockers"
	_ "github.com/karimra/gnmic/lockers/all"
)

func (c *Config) getLocker() error {
	c.Clustering.Locker = c.FileConfig.GetStringMap("clustering/locker")
	if len(c.Clustering.Locker) == 0 {
		return nil
	}
	if lockerType, ok := c.Clustering.Locker["type"]; ok {
		switch lockerType := lockerType.(type) {
		case string:
			if _, ok := lockers.Lockers[lockerType]; !ok {
				return errors.New("unknown locker type")
			}
		default:
			return errors.New("wrong locker type format")
		}
		return nil
	}
	return errors.New("missing locker type")
}
