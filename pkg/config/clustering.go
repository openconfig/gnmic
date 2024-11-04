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
	"time"

	"github.com/google/uuid"
	"github.com/openconfig/gnmic/pkg/api/types"
)

const (
	minTargetWatchTimer            = 20 * time.Second
	defaultTargetAssignmentTimeout = 10 * time.Second
	defaultServicesWatchTimer      = 1 * time.Minute
	defaultLeaderWaitTimer         = 5 * time.Second
)

type clustering struct {
	ClusterName             string                 `mapstructure:"cluster-name,omitempty" json:"cluster-name,omitempty" yaml:"cluster-name,omitempty"`
	InstanceName            string                 `mapstructure:"instance-name,omitempty" json:"instance-name,omitempty" yaml:"instance-name,omitempty"`
	ServiceAddress          string                 `mapstructure:"service-address,omitempty" json:"service-address,omitempty" yaml:"service-address,omitempty"`
	ServicesWatchTimer      time.Duration          `mapstructure:"services-watch-timer,omitempty" json:"services-watch-timer,omitempty" yaml:"services-watch-timer,omitempty"`
	TargetsWatchTimer       time.Duration          `mapstructure:"targets-watch-timer,omitempty" json:"targets-watch-timer,omitempty" yaml:"targets-watch-timer,omitempty"`
	TargetAssignmentTimeout time.Duration          `mapstructure:"target-assignment-timeout,omitempty" json:"target-assignment-timeout,omitempty" yaml:"target-assignment-timeout,omitempty"`
	LeaderWaitTimer         time.Duration          `mapstructure:"leader-wait-timer,omitempty" json:"leader-wait-timer,omitempty" yaml:"leader-wait-timer,omitempty"`
	Tags                    []string               `mapstructure:"tags,omitempty" json:"tags,omitempty" yaml:"tags,omitempty"`
	Locker                  map[string]interface{} `mapstructure:"locker,omitempty" json:"locker,omitempty" yaml:"locker,omitempty"`
	TLS                     *types.TLSConfig       `mapstructure:"tls,omitempty" json:"tls,omitempty" yaml:"tls,omitempty"`
}

func (c *Config) GetClustering() error {
	if !c.FileConfig.IsSet("clustering") {
		return nil
	}
	c.Clustering = new(clustering)
	c.Clustering.ClusterName = os.ExpandEnv(c.FileConfig.GetString("clustering/cluster-name"))
	c.Clustering.InstanceName = os.ExpandEnv(c.FileConfig.GetString("clustering/instance-name"))
	c.Clustering.ServiceAddress = os.ExpandEnv(c.FileConfig.GetString("clustering/service-address"))
	c.Clustering.TargetsWatchTimer = c.FileConfig.GetDuration("clustering/targets-watch-timer")
	c.Clustering.TargetAssignmentTimeout = c.FileConfig.GetDuration("clustering/target-assignment-timeout")
	c.Clustering.ServicesWatchTimer = c.FileConfig.GetDuration("clustering/services-watch-timer")
	c.Clustering.LeaderWaitTimer = c.FileConfig.GetDuration("clustering/leader-wait-timer")
	c.Clustering.Tags = c.FileConfig.GetStringSlice("clustering/tags")
	for i := range c.Clustering.Tags {
		c.Clustering.Tags[i] = os.ExpandEnv(c.Clustering.Tags[i])
	}
	if c.FileConfig.IsSet("clustering/tls") {
		c.Clustering.TLS = new(types.TLSConfig)
		c.Clustering.TLS.CaFile = os.ExpandEnv(c.FileConfig.GetString("clustering/tls/ca-file"))
		c.Clustering.TLS.CertFile = os.ExpandEnv(c.FileConfig.GetString("clustering/tls/cert-file"))
		c.Clustering.TLS.KeyFile = os.ExpandEnv(c.FileConfig.GetString("clustering/tls/key-file"))
		c.Clustering.TLS.SkipVerify = os.ExpandEnv(c.FileConfig.GetString("clustering/tls/skip-verify")) == trueString
		if err := c.APIServer.TLS.Validate(); err != nil {
			return fmt.Errorf("clustering TLS config error: %w", err)
		}
	}
	c.setClusteringDefaults()
	return c.getLocker()
}

func (c *Config) setClusteringDefaults() {
	// set $clustering.cluster-name to $cluster-name if it's empty string
	if c.Clustering.ClusterName == "" {
		c.Clustering.ClusterName = c.ClusterName
		// otherwise, set $cluster-name to $clustering.cluster-name
	} else {
		c.ClusterName = c.Clustering.ClusterName
	}
	// set clustering.instance-name to instance-name
	if c.Clustering.InstanceName == "" {
		if c.InstanceName != "" {
			c.Clustering.InstanceName = c.InstanceName
		} else {
			c.Clustering.InstanceName = "gnmic-" + uuid.New().String()
		}
	} else {
		c.InstanceName = c.Clustering.InstanceName
	}
	// the timers are set to less than the min allowed value,
	// make them default to that min value.
	if c.Clustering.TargetsWatchTimer < minTargetWatchTimer {
		c.Clustering.TargetsWatchTimer = minTargetWatchTimer
	}
	if c.Clustering.TargetAssignmentTimeout < defaultTargetAssignmentTimeout {
		c.Clustering.TargetAssignmentTimeout = defaultTargetAssignmentTimeout
	}
	if c.Clustering.ServicesWatchTimer <= defaultServicesWatchTimer {
		c.Clustering.ServicesWatchTimer = defaultServicesWatchTimer
	}
	if c.Clustering.LeaderWaitTimer <= defaultLeaderWaitTimer {
		c.Clustering.LeaderWaitTimer = defaultLeaderWaitTimer
	}
}
