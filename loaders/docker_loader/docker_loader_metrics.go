package docker_loader

import "github.com/prometheus/client_golang/prometheus"

var dockerLoaderLoadedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "docker_loader",
	Name:      "number_of_loaded_targets",
	Help:      "Number of new targets successfully loaded",
}, []string{"loader_type"})

var dockerLoaderDeletedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "docker_loader",
	Name:      "number_of_deleted_targets",
	Help:      "Number of targets successfully deleted",
}, []string{"loader_type"})

var dockerLoaderFailedListRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "docker_loader",
	Name:      "number_of_failed_docker_list",
	Help:      "Number of times a docker list failed",
}, []string{"loader_type", "error"})

var dockerLoaderListRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "docker_loader",
	Name:      "number_of_docker_list_total",
	Help:      "Number of times the loader sent a docker lits request",
}, []string{"loader_type"})

var dockerLoaderListRequestDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "docker_loader",
	Name:      "docker_list_duration_ns",
	Help:      "Duration of docker list request in ns",
}, []string{"loader_type"})

func initMetrics() {
	dockerLoaderLoadedTargets.WithLabelValues(loaderType).Set(0)
	dockerLoaderDeletedTargets.WithLabelValues(loaderType).Set(0)
	dockerLoaderFailedListRequests.WithLabelValues(loaderType, "").Add(0)
	dockerLoaderListRequestsTotal.WithLabelValues(loaderType).Add(0)
	dockerLoaderListRequestDuration.WithLabelValues(loaderType).Set(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	if reg == nil {
		return nil
	}
	initMetrics()
	var err error
	if err = reg.Register(dockerLoaderLoadedTargets); err != nil {
		return err
	}
	if err = reg.Register(dockerLoaderDeletedTargets); err != nil {
		return err
	}
	if err = reg.Register(dockerLoaderFailedListRequests); err != nil {
		return err
	}
	if err = reg.Register(dockerLoaderListRequestsTotal); err != nil {
		return err
	}
	if err = reg.Register(dockerLoaderListRequestDuration); err != nil {
		return err
	}
	return nil
}
