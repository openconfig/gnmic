// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"

	"github.com/openconfig/gnmic/pkg/lockers"
	_ "github.com/openconfig/gnmic/pkg/lockers/all"
)

func (c *Config) getLocker() error {
	if !c.FileConfig.IsSet("clustering/locker") {
		return errors.New("missing locker config")
	}
	c.Clustering.Locker = c.FileConfig.GetStringMap("clustering/locker")
	if len(c.Clustering.Locker) == 0 {
		return errors.New("missing locker config")
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
		expandMapEnv(c.Clustering.Locker)
		return nil
	}
	return errors.New("missing locker type")
}
