// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"strings"
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

var subscribeResponseFailedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "subscribe",
	Name:      "number_of_failed_subscribe_request_messages_total",
	Help:      "Total number of failed subscribe requests",
}, []string{"source", "subscription"})

// target
var targetUPMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "target",
	Name:      "up",
	Help:      "Has value 1 if the gNMI connection to the target is established; otherwise, 0.",
}, []string{"name"})

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

func (a *App) registerTargetMetrics() {
	err := a.reg.Register(targetUPMetric)
	if err != nil {
		a.Logger.Printf("failed to register target metric: %v", err)
	}
	a.configLock.RLock()
	for _, t := range a.Config.Targets {
		targetUPMetric.WithLabelValues(t.Name).Set(0)
	}
	a.configLock.RUnlock()
	go func() {
		ticker := time.NewTicker(clusterMetricsUpdatePeriod)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				ownTargets := make(map[string]string)
				if a.isLeader {
					lockedNodesPrefix := fmt.Sprintf("gnmic/%s/targets", a.Config.ClusterName)
					ctx, cancel := context.WithTimeout(a.ctx, clusterMetricsUpdatePeriod/2)
					lockedNodes, err := a.locker.List(ctx, lockedNodesPrefix)
					cancel()
					if err != nil {
						a.Logger.Printf("failed to get locked nodes key: %v", err)
					}
					for k, v := range lockedNodes {
						ownTargets[strings.TrimPrefix(k, lockedNodesPrefix+"/")] = v
					}
				}

				a.configLock.RLock()
				for _, tc := range a.Config.Targets {
					a.operLock.RLock()
					t, ok := a.Targets[tc.Name]
					a.operLock.RUnlock()
					if ok {
						switch t.ConnState() {
						case "IDLE", "READY":
							targetUPMetric.WithLabelValues(tc.Name).Set(1)
						default:
							targetUPMetric.WithLabelValues(tc.Name).Set(0)
						}
					} else {
						if a.isLeader {
							if ownTargets[tc.Name] == a.Config.Clustering.InstanceName {
								targetUPMetric.WithLabelValues(tc.Name).Set(0)
							}
						} else {
							targetUPMetric.WithLabelValues(tc.Name).Set(0)
						}
					}
				}
				a.configLock.RUnlock()
			}
		}
	}()
}

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
			if leader[leaderKey] == a.Config.Clustering.InstanceName {
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
				if v == a.Config.Clustering.InstanceName {
					numLockedNodes++
				}
			}
			clusterNumberOfLockedTargets.Set(float64(numLockedNodes))
		}
	}
}
