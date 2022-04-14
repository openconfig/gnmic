package app

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	clusterMetricsUpdatePeriod = 10 * time.Second
)

// subscribe
var subscribeResponseReceivedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "subscribe",
	Name:      "number_of_received_subscribe_response_messages_total",
	Help:      "Total number of received subscribe response messages",
}, []string{"source", "subscription"})

// cluster
var clusterNumberOfLockedTargets = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "cluster",
	Name:      "number_of_locked_targets",
	Help:      "number of locked targets",
})
var clusterIsLeader = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "cluster",
	Name:      "is_leader",
	Help:      "Has value 1 if this gnmic instance is the cluster leader, 0 otherwise",
})

func (a *App) startClusterMetrics() {
	if a.Config.APIServer == nil || !a.Config.APIServer.EnableMetrics || a.Config.Clustering == nil {
		return
	}
	var err error
	err = a.reg.Register(clusterNumberOfLockedTargets)
	if err != nil {
		a.Logger.Printf("failed to register metric: %v", err)
	}
	err = a.reg.Register(clusterIsLeader)
	if err != nil {
		a.Logger.Printf("failed to register metric: %v", err)
	}
	ticker := time.NewTicker(clusterMetricsUpdatePeriod)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(a.ctx, clusterMetricsUpdatePeriod/2)
			leaderKey := fmt.Sprintf("gnmic/%s/leader", a.Config.ClusterName)
			leader, err := a.locker.List(ctx, leaderKey)
			cancel()
			if err != nil {
				a.Logger.Printf("failed to get leader key: %v", err)
			}
			if leader[leaderKey] == a.Config.InstanceName {
				clusterIsLeader.Set(1)
			} else {
				clusterIsLeader.Set(0)
			}

			lockedNodesPrefix := fmt.Sprintf("gnmic/%s/targets", a.Config.ClusterName)
			ctx, cancel = context.WithTimeout(a.ctx, clusterMetricsUpdatePeriod/2)
			lockedNodes, err := a.locker.List(ctx, lockedNodesPrefix)
			cancel()
			if err != nil {
				a.Logger.Printf("failed to get locked nodes key: %v", err)
			}
			numLockedNodes := 0
			for _, v := range lockedNodes {
				if v == a.Config.InstanceName {
					numLockedNodes++
				}
			}
			clusterNumberOfLockedTargets.Set(float64(numLockedNodes))
		}
	}
}
