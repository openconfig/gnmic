package config

import (
	"os"
	"strings"
)

func envToMap() map[string]interface{} {
	m := map[string]interface{}{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, envPrefix) {
			continue
		}
		e = strings.ToLower(strings.Replace(e, envPrefix+"_", "", 1))
		pair := strings.SplitN(e, "=", 2)
		items := strings.Split(pair[0], "_")
		mergeMap(m, items, pair[1])
	}
	return m
}

func mergeMap(m map[string]interface{}, items []string, v interface{}) {
	nItems := len(items)
	if nItems == 0 {
		return
	}
	if nItems > 1 {
		if _, ok := m[items[0]]; !ok {
			m[items[0]] = map[string]interface{}{}
		}
		asMap, ok := m[items[0]].(map[string]interface{})
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

func expandMapEnv(m map[string]interface{}, except ...string) {
OUTER:
	for f := range m {
		switch v := m[f].(type) {
		case string:
			for _, e := range except {
				if f == e {
					continue OUTER
				}
			}
			m[f] = os.ExpandEnv(v)
		case map[string]interface{}:
			expandMapEnv(v)
			m[f] = v
		}
	}
}
