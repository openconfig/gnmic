package config

import (
	"errors"

	"github.com/karimra/gnmic/lockers"
	_ "github.com/karimra/gnmic/lockers/all"
)

func (c *Config) GetLocker() (map[string]interface{}, error) {
	lockerCfg := c.FileConfig.GetStringMap("locker")
	if lockerCfg == nil {
		return nil, nil
	}
	if lockerType, ok := lockerCfg["type"]; ok {
		switch lockerType := lockerType.(type) {
		case string:
			if _, ok := lockers.Lockers[lockerType]; !ok {
				return nil, errors.New("unknown locker type")
			}
		default:
			return nil, errors.New("wrong locker type format")
		}
		return lockerCfg, nil
	}
	return nil, errors.New("missing locker type")
}
