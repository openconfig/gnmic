package cache

import "log"

type Option func(*GnmiOutputCache)

func WithLogger(logger *log.Logger) Option {
	return func(gc *GnmiOutputCache) {
		gc.SetLogger(logger)
	}
}
