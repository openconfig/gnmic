package collector

import (
	"fmt"

	"github.com/karimra/gnmic/lockers"
)

func WithLocker(locker lockers.Locker) CollectorOption {
	return func(c *Collector) {
		c.locker = locker
	}
}

func (c *Collector) lockKey(s string) string {
	if s == "" {
		return s
	}
	return fmt.Sprintf("gnmic/%s/targets/%s", c.Config.ClusterName, s)
}
