package config

import (
	"os"
	"time"

	"github.com/google/uuid"
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
