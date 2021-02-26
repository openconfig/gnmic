package config

import (
	"time"

	"github.com/google/uuid"
)

const (
	defaultTargetWatchTimer        = 20 * time.Second
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
}

func (c *Config) GetClustering() error {
	if !c.FileConfig.IsSet("clustering") {
		return nil
	}
	c.Clustering = new(clustering)
	c.Clustering.ClusterName = c.FileConfig.GetString("clustering/cluster-name")
	c.Clustering.InstanceName = c.FileConfig.GetString("clustering/instance-name")
	c.Clustering.ServiceAddress = c.FileConfig.GetString("clustering/service-address")
	c.Clustering.TargetsWatchTimer = c.FileConfig.GetDuration("clustering/targets-watch-timer")
	c.Clustering.TargetAssignmentTimeout = c.FileConfig.GetDuration("clustering/target-assignment-timeout")
	c.Clustering.ServicesWatchTimer = c.FileConfig.GetDuration("clustering/services-watch-timer")
	c.Clustering.LeaderWaitTimer = c.FileConfig.GetDuration("clustering/leader-wait-timer")
	c.Clustering.Tags = c.FileConfig.GetStringSlice("clustering/tags")
	c.setClusteringDefaults()
	return c.getLocker()
}

func (c *Config) setClusteringDefaults() {
	if c.Clustering.ClusterName == "" {
		c.Clustering.ClusterName = c.LocalFlags.SubscribeClusterName
	}
	if c.Clustering.InstanceName == "" {
		if c.GlobalFlags.InstanceName != "" {
			c.Clustering.InstanceName = c.GlobalFlags.InstanceName
		} else {
			c.Clustering.InstanceName = "gnmic-" + uuid.New().String()
		}
	}
	if c.Clustering.TargetsWatchTimer < defaultTargetWatchTimer {
		c.Clustering.TargetsWatchTimer = defaultTargetWatchTimer
	}
	if c.Clustering.TargetAssignmentTimeout < defaultTargetAssignmentTimeout {
		c.Clustering.TargetAssignmentTimeout = defaultTargetAssignmentTimeout
	}
	if c.Clustering.ServicesWatchTimer <= 0 {
		c.Clustering.ServicesWatchTimer = defaultServicesWatchTimer
	}
	if c.Clustering.LeaderWaitTimer <= 0 {
		c.Clustering.LeaderWaitTimer = defaultLeaderWaitTimer
	}
}
