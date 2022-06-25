package cache

import "log"

type Option func(*GnmiCache)

func WithLogger(logger *log.Logger) Option {
	return func(gc *GnmiCache) {
		gc.SetLogger(logger)
	}
}
