package targets_manager

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	targetMetricsUpdatePeriod = 10 * time.Second
)

type targetConnectionState int

const (
	targetConnectionStateUnknown targetConnectionState = iota
	targetConnectionStateIdle
	targetConnectionStateConnecting
	targetConnectionStateReady
	targetConnectionStateTransientFailure
	targetConnectionStateShutdown
)

const (
	targetConnectionStateUnknownStr          = "UNKNOWN"
	targetConnectionStateIdleStr             = "IDLE"
	targetConnectionStateConnectingStr       = "CONNECTING"
	targetConnectionStateReadyStr            = "READY"
	targetConnectionStateTransientFailureStr = "TRANSIENT_FAILURE"
	targetConnectionStateShutdownStr         = "SHUTDOWN"
)

func targetConnectionStateFromStr(str string) targetConnectionState {
	switch str {
	case targetConnectionStateUnknownStr:
		return targetConnectionStateUnknown
	case targetConnectionStateIdleStr:
		return targetConnectionStateIdle
	case targetConnectionStateConnectingStr:
		return targetConnectionStateConnecting
	case targetConnectionStateReadyStr:
		return targetConnectionStateReady
	case targetConnectionStateTransientFailureStr:
		return targetConnectionStateTransientFailure
	case targetConnectionStateShutdownStr:
		return targetConnectionStateShutdown
	}
	return targetConnectionStateUnknown
}

func (tcs targetConnectionState) String() string {
	switch tcs {
	case targetConnectionStateUnknown:
		return targetConnectionStateUnknownStr
	case targetConnectionStateIdle:
		return targetConnectionStateIdleStr
	case targetConnectionStateConnecting:
		return targetConnectionStateConnectingStr
	case targetConnectionStateReady:
		return targetConnectionStateReadyStr
	case targetConnectionStateTransientFailure:
		return targetConnectionStateTransientFailureStr
	case targetConnectionStateShutdown:
		return targetConnectionStateShutdownStr
	}
	return ""
}

type targetsStats struct {
	subscribeResponseReceived *prometheus.CounterVec
	droppedSubscribeResponses *prometheus.CounterVec
	subscriptionFailedCount   *prometheus.CounterVec
	targetUPMetric            *prometheus.GaugeVec
	targetConnStateMetric     *prometheus.GaugeVec
	targetGrpcStateMetric     *prometheus.GaugeVec
}

const (
	subscriptionRequestErrorTypeUnknown string = "UNKNOWN"
	subscriptionRequestErrorTypeCONFIG  string = "CONFIG_ERROR"
	subscriptionRequestErrorTypeGRPC    string = "GRPC_ERROR"
)

func newTargetsStats() *targetsStats {
	return &targetsStats{
		subscribeResponseReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "gnmic",
			Subsystem: "targets",
			Name:      "subscribe_response_received_count",
			Help:      "Number of subscribe responses received",
		}, []string{"target", "subscription"}),
		droppedSubscribeResponses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "gnmic",
			Subsystem: "targets",
			Name:      "dropped_subscribe_responses_count",
			Help:      "Number of dropped subscribe responses",
		}, []string{"target", "subscription"}),
		subscriptionFailedCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "gnmic",
			Subsystem: "subscribe",
			Name:      "number_of_failed_subscribe_request_messages_total",
			Help:      "Total number of failed subscribe requests",
		}, []string{"target", "subscription", "error_type"}),
		targetUPMetric: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "gnmic",
			Subsystem: "target",
			Name:      "up",
			Help:      "Has value 1 if the gNMI target is configured; otherwise, 0.",
		}, []string{"name"}),
		targetConnStateMetric: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "gnmic",
			Subsystem: "target",
			Name:      "connection_state",
			Help:      "The current gRPC connection state to the target. The value can be one of the following: 0(UNKNOWN), 1 (IDLE), 2 (CONNECTING), 3 (READY), 4 (TRANSIENT_FAILURE), or 5 (SHUTDOWN).",
		}, []string{"name"}),
		// targetGrpcStateMetric tracks the last gRPC status code received from each target.
		// See https://pkg.go.dev/google.golang.org/grpc/codes for the full reference.
		targetGrpcStateMetric: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "gnmic",
			Subsystem: "target",
			Name:      "grpc_status_code",
			Help: "The last gRPC status code received from the target. Default is 2 (Unknown) until the first response is received. " +
				"See https://pkg.go.dev/google.golang.org/grpc/codes. " +
				"Values: 0=OK, 1=Canceled, 2=Unknown, 3=InvalidArgument, 4=DeadlineExceeded, 5=NotFound, " +
				"6=AlreadyExists, 7=PermissionDenied, 8=ResourceExhausted, 9=FailedPrecondition, 10=Aborted, " +
				"11=OutOfRange, 12=Unimplemented, 13=Internal, 14=Unavailable, 15=DataLoss, 16=Unauthenticated.",
		}, []string{"name"}),
	}
}

func (tm *TargetsManager) registerMetrics() {
	tm.reg.MustRegister(tm.stats.targetUPMetric)
	tm.reg.MustRegister(tm.stats.targetConnStateMetric)
	tm.reg.MustRegister(tm.stats.subscribeResponseReceived)
	tm.reg.MustRegister(tm.stats.droppedSubscribeResponses)
	tm.reg.MustRegister(tm.stats.subscriptionFailedCount)
	tm.reg.MustRegister(tm.stats.targetGrpcStateMetric)

	tm.mu.RLock()
	for _, mt := range tm.targets {
		tm.updateTargetMetrics(mt)
	}

	tm.mu.RUnlock()
	go func() {
		ticker := time.NewTicker(targetMetricsUpdatePeriod)
		defer ticker.Stop()
		for {
			select {
			case <-tm.ctx.Done():
				return
			case <-ticker.C:
				tm.mu.RLock()
				for _, mt := range tm.targets {
					tm.updateTargetMetrics(mt)
				}
				tm.mu.RUnlock()
			}
		}
	}()
}

func (tm *TargetsManager) updateTargetMetrics(mt *ManagedTarget) {
	if mt.T == nil {
		tm.stats.targetUPMetric.WithLabelValues(mt.Name).Set(0)
		tm.stats.targetConnStateMetric.WithLabelValues(mt.Name).Set(0)
		tm.stats.targetGrpcStateMetric.WithLabelValues(mt.Name).Set(2) // Unknown until first response
		return
	}
	tm.stats.targetUPMetric.WithLabelValues(mt.Name).Set(1)
	targetConnState := targetConnectionStateFromStr(mt.T.ConnState())
	tm.stats.targetConnStateMetric.WithLabelValues(mt.Name).Set(float64(targetConnState))
}
