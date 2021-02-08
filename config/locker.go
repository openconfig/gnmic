package config

import (
	"errors"

	"github.com/karimra/gnmic/lockers"
	_ "github.com/karimra/gnmic/lockers/all"
)

func (c *Config) GetLocker() (map[string]interface{}, error) {
	c.Locker = c.FileConfig.GetStringMap("locker")
	if len(c.Locker) == 0 {
		return nil, nil
	}
	if lockerType, ok := c.Locker["type"]; ok {
		switch lockerType := lockerType.(type) {
		case string:
			if _, ok := lockers.Lockers[lockerType]; !ok {
				return nil, errors.New("unknown locker type")
			}
		default:
			return nil, errors.New("wrong locker type format")
		}
		return c.Locker, nil
	}
	return nil, errors.New("missing locker type")
}
