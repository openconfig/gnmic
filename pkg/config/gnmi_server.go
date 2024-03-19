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
	"os"
	"strconv"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/cache"
	"google.golang.org/grpc/keepalive"
)

const (
	defaultAddress           = ":57400"
	defaultMaxSubscriptions  = 64
	defaultMaxUnaryRPC       = 64
	minimumSampleInterval    = 1 * time.Millisecond
	defaultSampleInterval    = 1 * time.Second
	minimumHeartbeatInterval = 1 * time.Second
	//
	defaultServiceRegistrationAddress = "localhost:8500"
	defaultRegistrationCheckInterval  = 5 * time.Second
	defaultMaxServiceFail             = 3
)

type gnmiServer struct {
	Address               string               `mapstructure:"address,omitempty" json:"address,omitempty"`
	MinSampleInterval     time.Duration        `mapstructure:"min-sample-interval,omitempty" json:"min-sample-interval,omitempty"`
	DefaultSampleInterval time.Duration        `mapstructure:"default-sample-interval,omitempty" json:"default-sample-interval,omitempty"`
	MinHeartbeatInterval  time.Duration        `mapstructure:"min-heartbeat-interval,omitempty" json:"min-heartbeat-interval,omitempty"`
	MaxSubscriptions      int64                `mapstructure:"max-subscriptions,omitempty" json:"max-subscriptions,omitempty"`
	MaxUnaryRPC           int64                `mapstructure:"max-unary-rpc,omitempty" json:"max-unary-rpc,omitempty"`
	MaxRecvMsgSize        int                  `mapstructure:"max-recv-msg-size,omitempty" json:"max-recv-msg-size,omitempty"`
	MaxSendMsgSize        int                  `mapstructure:"max-send-msg-size,omitempty" json:"max-send-msg-size,omitempty"`
	MaxConcurrentStreams  uint32               `mapstructure:"max-concurrent-streams,omitempty" json:"max-concurrent-streams,omitempty"`
	TCPKeepalive          time.Duration        `mapstructure:"tcp-keepalive,omitempty" json:"tcp-keepalive,omitempty"`
	GRPCKeepalive         *grpcKeepaliveConfig `mapstructure:"grpc-keepalive,omitempty" json:"grpc-keepalive,omitempty"`
	RateLimit             int64                `mapstructure:"rate-limit,omitempty" json:"rate-limit,omitempty"`
	TLS                   *types.TLSConfig     `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	EnableMetrics         bool                 `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	Debug                 bool                 `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	// ServiceRegistration
	ServiceRegistration *serviceRegistration `mapstructure:"service-registration,omitempty" json:"service-registration,omitempty"`
	// cache config
	Cache *cache.Config `mapstructure:"cache,omitempty" json:"cache,omitempty"`
}

type serviceRegistration struct {
	Address       string        `mapstructure:"address,omitempty" json:"address,omitempty"`
	Datacenter    string        `mapstructure:"datacenter,omitempty" json:"datacenter,omitempty"`
	Username      string        `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password      string        `mapstructure:"password,omitempty" json:"password,omitempty"`
	Token         string        `mapstructure:"token,omitempty" json:"token,omitempty"`
	Name          string        `mapstructure:"name,omitempty" json:"name,omitempty"`
	CheckInterval time.Duration `mapstructure:"check-interval,omitempty" json:"check-interval,omitempty"`
	MaxFail       int           `mapstructure:"max-fail,omitempty" json:"max-fail,omitempty"`
	Tags          []string      `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	//
	DeregisterAfter string `mapstructure:"-" json:"-"`
}

// from keepalive.ServerParameters
type grpcKeepaliveConfig struct {
	// MaxConnectionIdle is a duration for the amount of time after which an
	// idle connection would be closed by sending a GoAway. Idleness duration is
	// defined since the most recent time the number of outstanding RPCs became
	// zero or the connection establishment.
	MaxConnectionIdle time.Duration `mapstructure:"max-connection-idle,omitempty"` // The current default value is infinity.
	// MaxConnectionAge is a duration for the maximum amount of time a
	// connection may exist before it will be closed by sending a GoAway. A
	// random jitter of +/-10% will be added to MaxConnectionAge to spread out
	// connection storms.
	MaxConnectionAge time.Duration `mapstructure:"max-connection-age,omitempty"` // The current default value is infinity.
	// MaxConnectionAgeGrace is an additive period after MaxConnectionAge after
	// which the connection will be forcibly closed.
	MaxConnectionAgeGrace time.Duration `mapstructure:"max-connection-age-grace,omitempty"` // The current default value is infinity.
	// After a duration of this time if the server doesn't see any activity it
	// pings the client to see if the transport is still alive.
	// If set below 1s, a minimum value of 1s will be used instead.
	Time time.Duration `mapstructure:"time,omitempty"` // The current default value is 2 hours.
	// After having pinged for keepalive check, the server waits for a duration
	// of Timeout and if no activity is seen even after that the connection is
	// closed.
	Timeout time.Duration `mapstructure:"timeout,omitempty"` // The current default value is 20 seconds.
}

func (gkc *grpcKeepaliveConfig) Convert() *keepalive.ServerParameters {
	if gkc == nil {
		return nil
	}
	return &keepalive.ServerParameters{
		MaxConnectionIdle:     gkc.MaxConnectionIdle,
		MaxConnectionAge:      gkc.MaxConnectionAge,
		MaxConnectionAgeGrace: gkc.MaxConnectionAgeGrace,
		Time:                  gkc.Time,
		Timeout:               gkc.Timeout,
	}
}

func (c *Config) GetGNMIServer() error {
	if !c.FileConfig.IsSet("gnmi-server") {
		return nil
	}
	c.GnmiServer = new(gnmiServer)
	c.GnmiServer.Address = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/address"))

	maxSubVal := os.ExpandEnv(c.FileConfig.GetString("gnmi-server/max-subscriptions"))
	if maxSubVal != "" {
		maxSub, err := strconv.Atoi(maxSubVal)
		if err != nil {
			return err
		}
		c.GnmiServer.MaxSubscriptions = int64(maxSub)
	}
	maxRPCVal := os.ExpandEnv(c.FileConfig.GetString("gnmi-server/max-unary-rpc"))
	if maxRPCVal != "" {
		maxUnaryRPC, err := strconv.Atoi(os.ExpandEnv(c.FileConfig.GetString("gnmi-server/max-unary-rpc")))
		if err != nil {
			return err
		}
		c.GnmiServer.MaxUnaryRPC = int64(maxUnaryRPC)
	}
	if c.FileConfig.IsSet("gnmi-server/tls") {
		c.GnmiServer.TLS = new(types.TLSConfig)
		c.GnmiServer.TLS.CaFile = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/tls/ca-file"))
		c.GnmiServer.TLS.CertFile = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/tls/cert-file"))
		c.GnmiServer.TLS.KeyFile = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/tls/key-file"))
		c.GnmiServer.TLS.ClientAuth = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/tls/client-auth"))
		if err := c.GnmiServer.TLS.Validate(); err != nil {
			return fmt.Errorf("gnmi-server TLS config error: %w", err)
		}
	}

	c.GnmiServer.EnableMetrics = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/enable-metrics")) == trueString
	c.GnmiServer.Debug = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/debug")) == trueString
	c.setGnmiServerDefaults()

	if c.FileConfig.IsSet("gnmi-server/service-registration") {
		c.GnmiServer.ServiceRegistration = new(serviceRegistration)
		c.GnmiServer.ServiceRegistration.Address = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/service-registration/address"))
		c.GnmiServer.ServiceRegistration.Datacenter = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/service-registration/datacenter"))
		c.GnmiServer.ServiceRegistration.Username = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/service-registration/username"))
		c.GnmiServer.ServiceRegistration.Password = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/service-registration/password"))
		c.GnmiServer.ServiceRegistration.Token = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/service-registration/token"))
		c.GnmiServer.ServiceRegistration.Name = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/service-registration/name"))
		c.GnmiServer.ServiceRegistration.CheckInterval = c.FileConfig.GetDuration("gnmi-server/service-registration/check-interval")
		c.GnmiServer.ServiceRegistration.MaxFail = c.FileConfig.GetInt("gnmi-server/service-registration/max-fail")
		c.GnmiServer.ServiceRegistration.Tags = c.FileConfig.GetStringSlice("gnmi-server/service-registration/tags")
		c.setGnmiServerServiceRegistrationDefaults()
	}

	if c.FileConfig.IsSet("gnmi-server/cache") {
		c.GnmiServer.Cache = new(cache.Config)
		c.GnmiServer.Cache.Type = cache.CacheType(os.ExpandEnv(c.FileConfig.GetString("gnmi-server/cache/type")))
		c.GnmiServer.Cache.Address = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/cache/address"))
		c.GnmiServer.Cache.Timeout = c.FileConfig.GetDuration("gnmi-server/cache/timeout")
		c.GnmiServer.Cache.Expiration = c.FileConfig.GetDuration("gnmi-server/cache/expiration")
		c.GnmiServer.Cache.Debug = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/cache/debug")) == trueString
		c.GnmiServer.Cache.Username = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/cache/username"))
		c.GnmiServer.Cache.Password = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/cache/password"))
		//
		c.GnmiServer.Cache.MaxBytes = c.FileConfig.GetInt64("gnmi-server/cache/max-bytes")
		c.GnmiServer.Cache.MaxMsgsPerSubscription = c.FileConfig.GetInt64("gnmi-server/cache/max-msgs-per-subscription")
		//
		c.GnmiServer.Cache.FetchBatchSize = c.FileConfig.GetInt("gnmi-server/cache/fetch-batch-size")
		c.GnmiServer.Cache.FetchWaitTime = c.FileConfig.GetDuration("gnmi-server/cache/fetch-wait-time")
	}
	return nil
}

func (c *Config) setGnmiServerDefaults() {
	if c.GnmiServer.Address == "" {
		c.GnmiServer.Address = defaultAddress
	}
	if c.GnmiServer.MaxSubscriptions <= 0 {
		c.GnmiServer.MaxSubscriptions = defaultMaxSubscriptions
	}
	if c.GnmiServer.MaxUnaryRPC <= 0 {
		c.GnmiServer.MaxUnaryRPC = defaultMaxUnaryRPC
	}
	if c.GnmiServer.MinSampleInterval <= 0 {
		c.GnmiServer.MinSampleInterval = minimumSampleInterval
	}
	if c.GnmiServer.DefaultSampleInterval <= 0 {
		c.GnmiServer.DefaultSampleInterval = defaultSampleInterval
	}
	if c.GnmiServer.MinHeartbeatInterval <= 0 {
		c.GnmiServer.MinHeartbeatInterval = minimumHeartbeatInterval
	}
}

func (c *Config) setGnmiServerServiceRegistrationDefaults() {
	if c.GnmiServer.ServiceRegistration.Address == "" {
		c.GnmiServer.ServiceRegistration.Address = defaultServiceRegistrationAddress
	}
	if c.GnmiServer.ServiceRegistration.CheckInterval <= 5*time.Second {
		c.GnmiServer.ServiceRegistration.CheckInterval = defaultRegistrationCheckInterval
	}
	if c.GnmiServer.ServiceRegistration.MaxFail <= 0 {
		c.GnmiServer.ServiceRegistration.MaxFail = defaultMaxServiceFail
	}
	deregisterTimer := c.GnmiServer.ServiceRegistration.CheckInterval * time.Duration(c.GnmiServer.ServiceRegistration.MaxFail)
	c.GnmiServer.ServiceRegistration.DeregisterAfter = deregisterTimer.String()
}
