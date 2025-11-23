package targets_manager

import "github.com/prometheus/client_golang/prometheus"

var subscribeResponseReceivedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "targets",
	Name:      "subscribe_response_received_count",
	Help:      "Number of subscribe responses received",
}, []string{"target", "subscription"})
