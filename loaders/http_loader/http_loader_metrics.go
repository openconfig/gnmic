package http_loader

import "github.com/prometheus/client_golang/prometheus"

var httpLoaderLoadedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "http_loader",
	Name:      "number_of_loaded_targets",
	Help:      "Number of new targets successfully loaded",
}, []string{"loader_type"})

var httpLoaderDeletedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "http_loader",
	Name:      "number_of_deleted_targets",
	Help:      "Number of targets successfully deleted",
}, []string{"loader_type"})

var httpLoaderFailedGetRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "http_loader",
	Name:      "number_of_failed_http_requests",
	Help:      "Number of times the http Get request failed",
}, []string{"loader_type", "error"})

var httpLoaderGetRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "http_loader",
	Name:      "number_of_http_requests_total",
	Help:      "Number of times the loader sent an HTTP request",
}, []string{"loader_type"})

var httpLoaderGetRequestDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "http_loader",
	Name:      "http_request_duration_ns",
	Help:      "Duration of http request in ns",
}, []string{"loader_type"})

func initMetrics() {
	httpLoaderLoadedTargets.WithLabelValues(loaderType).Set(0)
	httpLoaderDeletedTargets.WithLabelValues(loaderType).Set(0)
	httpLoaderFailedGetRequests.WithLabelValues(loaderType, "").Add(0)
	httpLoaderGetRequestsTotal.WithLabelValues(loaderType).Add(0)
	httpLoaderGetRequestDuration.WithLabelValues(loaderType).Set(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(httpLoaderLoadedTargets); err != nil {
		return err
	}
	if err = reg.Register(httpLoaderDeletedTargets); err != nil {
		return err
	}
	if err = reg.Register(httpLoaderFailedGetRequests); err != nil {
		return err
	}
	if err = reg.Register(httpLoaderGetRequestsTotal); err != nil {
		return err
	}
	if err = reg.Register(httpLoaderGetRequestDuration); err != nil {
		return err
	}
	return nil
}
