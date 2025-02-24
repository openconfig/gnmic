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
	"fmt"
	"os"
	"strings"

	"github.com/openconfig/gnmic/pkg/loaders"
	_ "github.com/openconfig/gnmic/pkg/loaders/all"
)

func (c *Config) GetLoader() error {
	if c.GlobalFlags.TargetsFile != "" {
		c.Loader = map[string]interface{}{
			"type": "file",
			"path": c.GlobalFlags.TargetsFile,
		}
		return nil
	}

	c.Loader = c.FileConfig.GetStringMap("loader")
	for k, v := range c.Loader {
		c.Loader[k] = convert(v)
	}

	if len(c.Loader) == 0 {
		return nil
	}
	if _, ok := c.Loader["type"]; !ok {
		return errors.New("missing type field under loader configuration")
	}
	if lds, ok := c.Loader["type"].(string); ok {
		for _, lt := range loaders.LoadersTypes {
			if lt == lds {
				expandMapEnv(c.Loader, func(k, v string) string {
					if k == "password" {
						if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
							return os.ExpandEnv(v)
						}
						return v
					}
					return os.ExpandEnv(v)
				})
				return nil
			}
		}
		return fmt.Errorf("unknown loader type %q", lds)
	}
	return fmt.Errorf("field 'type' not a string, found a %T", c.Loader["type"])
}
