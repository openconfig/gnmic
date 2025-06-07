// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func envToMap() map[string]any {
	m := map[string]any{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, envPrefix) {
			continue
		}
		pair := strings.SplitN(e, "=", 2)
		if len(pair) < 2 {
			continue // malformed env var
		}
		pair[0] = strings.ToLower(strings.TrimPrefix(pair[0], envPrefix+"_"))
		items := strings.Split(pair[0], "_")
		mergeMap(m, items, pair[1])
	}
	return m
}

func mergeMap(m map[string]any, items []string, v any) {
	nItems := len(items)
	if nItems == 0 {
		return
	}
	if nItems > 1 {
		if _, ok := m[items[0]]; !ok {
			m[items[0]] = map[string]any{}
		}
		asMap, ok := m[items[0]].(map[string]any)
		if !ok {
			return
		}
		mergeMap(asMap, items[1:], v)
		v = asMap
	}
	m[items[0]] = v
}

func (c *Config) mergeEnvVars() {
	envs := envToMap()
	if c.GlobalFlags.Debug {
		c.logger.Printf("merging env vars: %+v", envs)
	}
	c.FileConfig.MergeConfigMap(envs)
}

func (c *Config) SetGlobalsFromEnv(cmd *cobra.Command) {
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		// expand password and token global attr only if they start with '$'
		if f.Name == "password" || f.Name == "token" {
			if !f.Changed && c.FileConfig.IsSet(f.Name) {
				val := c.FileConfig.GetString(f.Name)
				if strings.HasPrefix(val, "$") {
					c.setFlagValue(cmd, f.Name, val)
				}
			}
			return
		}
		// other global flags
		if !f.Changed && c.FileConfig.IsSet(f.Name) {
			if val := os.ExpandEnv(c.FileConfig.GetString(f.Name)); val != "" {
				c.setFlagValue(cmd, f.Name, val)
			}
		}
	})
}

func expandMapEnv(m map[string]interface{}, fn func(string, string) string) {
	for f := range m {
		switch v := m[f].(type) {
		case string:
			m[f] = fn(f, v)
		case map[string]interface{}:
			expandMapEnv(v, fn)
			m[f] = v
		case []any:
			for i, item := range v {
				switch item := item.(type) {
				case string:
					v[i] = fn(f, item)
				case map[string]interface{}:
					expandMapEnv(item, fn)
				case []any:
					expandSliceEnv(f, item, fn)
				}
			}
			m[f] = v
		}
	}
}

func expandSliceEnv(parent string, s []any, fn func(string, string) string) {
	for i, item := range s {
		switch item := item.(type) {
		case string:
			s[i] = fn(parent, item)
		case map[string]interface{}:
			expandMapEnv(item, fn)
		case []any:
			expandSliceEnv("", item, fn)
		}
	}
}

func expandExcept(except ...string) func(string, string) string {
	return func(k, v string) string {
		for _, e := range except {
			if k == e {
				return v
			}
		}
		return os.ExpandEnv(v)
	}
}

func expandAll() func(string, string) string {
	return expandExcept()
}
