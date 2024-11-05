// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"

	"github.com/openconfig/gnmic/pkg/actions"
)

func (c *Config) GetActions() (map[string]map[string]interface{}, error) {
	for name, actc := range c.FileConfig.GetStringMap("actions") {
		switch actc := actc.(type) {
		case map[string]interface{}:
			c.logger.Printf("validating action %q config", name)
			err := c.validateActionsConfig(actc)
			if err != nil {
				return nil, err
			}
			// set action name if not configured
			if cname, ok := actc["name"]; !ok || cname == "" {
				actc["name"] = name
			}
			for nn, a := range actc {
				actc[nn] = convert(a)
			}
			c.Actions[name] = actc
		case nil:
			return nil, fmt.Errorf("empty action %q config", name)
		default:
			c.logger.Printf("malformed action config, %+v", actc)
			return nil, fmt.Errorf("malformed action config, got %T", actc)
		}
	}
	for n := range c.Actions {
		expandMapEnv(c.Actions[n],
			expandExcept(
				"target", "paths", "values", // gnmi action templates
				"url", "body", // http action templates
				"template", // template action templates
			))
	}
	if c.Debug {
		c.logger.Printf("actions: %+v", c.Actions)
	}
	return c.Actions, nil
}

func (c *Config) validateActionsConfig(acfg map[string]interface{}) error {
	if aType, ok := acfg["type"]; ok {
		switch aType := aType.(type) {
		case string:
			if !strInlist(aType, actions.ActionTypes) {
				return fmt.Errorf("unknown action type: %s, must be one of %q", aType, actions.ActionTypes)
			}
		default:
			return fmt.Errorf("unexpected action type variable type, expecting string, got %T", aType)
		}
		return nil
	}
	return fmt.Errorf("missing action type under %+v", acfg)
}
