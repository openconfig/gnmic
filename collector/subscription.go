package collector

import "time"

type Subscription struct {
	Name              string
	Paths             []string
	Mode              string
	SampleInterval    time.Duration
	HeartbeatInterval time.Duration
}
