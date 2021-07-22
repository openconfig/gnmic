package config

import (
	"os"
	"strconv"
)

const (
	defaultAddress          = ":57400"
	defaultMaxSubscriptions = 64
	defaultMaxUnaryRPC      = 64
)

type gnmiServer struct {
	Address          string `mapstructure:"address,omitempty"`
	MaxSubscriptions int64  `mapstructure:"max-subscriptions,omitempty"`
	MaxUnaryRPC      int64  `mapstructure:"max-unary-rpc,omitempty"`
	// TLS
	SkipVerify bool   `mapstructure:"skip-verify,omitempty"`
	CaFile     string `mapstructure:"ca-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty"`
	//
	EnableMetrics bool `mapstructure:"enable-metrics,omitempty"`
	Debug         bool `mapstructure:"debug,omitempty"`
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
}
