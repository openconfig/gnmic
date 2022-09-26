package cache

import "log"

type Option func(Cache)

func WithLogger(logger *log.Logger) Option {
	return func(c Cache) {
		c.SetLogger(logger)
	}
}
