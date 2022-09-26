package cache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

type CacheType string

const (
	cacheType_OC    CacheType = "oc"
	cacheType_Redis CacheType = "redis"
	cacheType_NATS  CacheType = "nats"
	cacheType_JS    CacheType = "jetstream"
)

const (
	ReadMode_Once           = "once"
	ReadMode_StreamOnChange = "stream_on_change"
	ReadMode_StreamSample   = "stream_sample"
)

type Cache interface {
	// Write inserts the proto.Message (SubscribeResponse) into the cache under a subscription called `sub`
	Write(ctx context.Context, sub string, m proto.Message)
	// Read entries from the local cache, return the entries grouped by subscription name.
	Read() (map[string][]*gnmi.Notification, error)
	// Subscribes to the local cache and returns the notification over a channel
	Subscribe(ctx context.Context, so *ReadOpts) chan *Notification
	// Stops the cache
	Stop()
	// DeleteTarget deletes the target from the cache by name
	DeleteTarget(name string)
	// SetLogger sets a logger for the cache
	SetLogger(l *log.Logger)
}

type Config struct {
	Type       CacheType     `mapstructure:"type,omitempty" json:"type,omitempty"`
	Address    string        `mapstructure:"address,omitempty" json:"address,omitempty"`
	Timeout    time.Duration `mapstructure:"timeout,omitempty" json:"timeout,omitempty"`
	Expiration time.Duration `mapstructure:"expiration,omitempty" json:"expiration,omitempty"`
	Debug      bool          `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	// NATS, JS and Redis cfg options
	Username string `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password string `mapstructure:"password,omitempty" json:"password,omitempty"`

	// JS cfg options
	MaxBytes               int64         `mapstructure:"max-bytes,omitempty" json:"max-bytes,omitempty"`
	MaxMsgsPerSubscription int64         `mapstructure:"max-msgs-per-subscription,omitempty" json:"max-msgs-per-subscription,omitempty"`
	FetchBatchSize         int           `mapstructure:"fetch-batch-size,omitempty" json:"fetch-batch-size,omitempty"`
	FetchWaitTime          time.Duration `mapstructure:"fetch-wait-time,omitempty" json:"fetch-wait-time,omitempty"`
}

func (c *Config) setDefaults() {
	if c.Address == "" {
		switch c.Type {
		case cacheType_Redis:
			c.Address = defaultRedisAddress
		case cacheType_JS, cacheType_NATS:
			c.Address = defaultNATSAddress
		}
	}
	if c.Timeout == 0 {
		c.Timeout = defaultTimeout
	}
	if c.Expiration <= 0 {
		c.Expiration = defaultExpiration
	}

	if c.Type != cacheType_JS {
		return
	}

	if c.MaxMsgsPerSubscription <= 0 {
		c.MaxMsgsPerSubscription = defaultMaxMsgs
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = defaultMaxBytes
	}
	if c.FetchBatchSize <= 0 {
		c.FetchBatchSize = defaultFetchBatchSize
	}
	if c.FetchWaitTime <= 0 {
		c.FetchWaitTime = defaultFetchWaitTime
	}
}

func New(c *Config, opts ...Option) (Cache, error) {
	if c == nil {
		c = &Config{Type: cacheType_OC}
	}
	if c.Type == "" {
		c.Type = cacheType_OC
	}
	switch c.Type {
	case cacheType_OC:
		return newGNMICache(c, "", opts...), nil
	case cacheType_NATS:
		return newNATSCache(c, opts...)
	case cacheType_JS:
		return newJetStreamCache(c, opts...)
	case cacheType_Redis:
		return newRedisCache(c, opts...)
	default:
		return nil, fmt.Errorf("unknown cache type: %q", c.Type)
	}
}

type ReadOpts struct {
	Subscription      string
	Target            string
	Paths             []*gnmi.Path
	Mode              string
	SampleInterval    time.Duration
	HeartbeatInterval time.Duration
	SuppressRedundant bool
	UpdatesOnly       bool
	OverrideTS        bool

	m        *sync.RWMutex
	lastSent map[string]*gnmi.TypedValue
}

func (ro *ReadOpts) setDefaults() {
	if ro.Target == "" {
		ro.Target = "*"
	}
	if ro.Mode == "" {
		ro.Mode = ReadMode_StreamOnChange
	}
	if len(ro.Paths) == 0 {
		ro.Paths = []*gnmi.Path{{}}
	}
	if ro.Mode == ReadMode_StreamSample && ro.SampleInterval <= 0 {
		ro.SampleInterval = 10 * time.Second
	}
	if ro.SuppressRedundant {
		ro.m = new(sync.RWMutex)
		ro.lastSent = make(map[string]*gnmi.TypedValue)
	}
}

type Notification struct {
	Name         string
	Notification *gnmi.Notification
	Err          error
}
