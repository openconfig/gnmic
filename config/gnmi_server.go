package config

import (
	"os"
	"strconv"
	"time"
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
	Address               string        `mapstructure:"address,omitempty" json:"address,omitempty"`
	MinSampleInterval     time.Duration `mapstructure:"min-sample-interval,omitempty" json:"min-sample-interval,omitempty"`
	DefaultSampleInterval time.Duration `mapstructure:"default-sample-interval,omitempty" json:"default-sample-interval,omitempty"`
	MinHeartbeatInterval  time.Duration `mapstructure:"min-heartbeat-interval,omitempty" json:"min-heartbeat-interval,omitempty"`
	MaxSubscriptions      int64         `mapstructure:"max-subscriptions,omitempty" json:"max-subscriptions,omitempty"`
	MaxUnaryRPC           int64         `mapstructure:"max-unary-rpc,omitempty" json:"max-unary-rpc,omitempty"`
	// TLS
	SkipVerify bool   `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty"`
	CaFile     string `mapstructure:"ca-file,omitempty" json:"ca-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty" json:"cert-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty" json:"key-file,omitempty"`
	//
	EnableMetrics bool `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	Debug         bool `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	// ServiceRegistration
	ServiceRegistration *serviceRegistration `mapstructure:"service-registration,omitempty" json:"service-registration,omitempty"`
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

	c.GnmiServer.SkipVerify = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/skip-verify")) == "true"
	c.GnmiServer.CaFile = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/ca-file"))
	c.GnmiServer.CertFile = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/cert-file"))
	c.GnmiServer.KeyFile = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/key-file"))

	c.GnmiServer.EnableMetrics = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/enable-metrics")) == "true"
	c.GnmiServer.Debug = os.ExpandEnv(c.FileConfig.GetString("gnmi-server/debug")) == "true"
	c.setGnmiServerDefaults()

	if !c.FileConfig.IsSet("gnmi-server/service-registration") {
		return nil
	}
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
