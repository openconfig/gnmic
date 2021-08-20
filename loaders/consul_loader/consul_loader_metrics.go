package consul_loader

import "github.com/prometheus/client_golang/prometheus"

var consulLoaderLoadedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "consul_loader",
	Name:      "number_of_loaded_targets",
	Help:      "Number of new targets successfully loaded",
}, []string{"loader_type"})

var consulLoaderDeletedTargets = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "consul_loader",
	Name:      "number_of_deleted_targets",
	Help:      "Number of targets successfully deleted",
}, []string{"loader_type"})

var consulLoaderWatchError = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "consul_loader",
	Name:      "number_of_watch_errors",
	Help:      "Number of watch errors",
}, []string{"loader_type", "error"})

func initMetrics() {
	consulLoaderLoadedTargets.WithLabelValues(loaderType).Set(0)
	consulLoaderDeletedTargets.WithLabelValues(loaderType).Set(0)
	consulLoaderWatchError.WithLabelValues(loaderType, "").Add(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(consulLoaderLoadedTargets); err != nil {
		return err
	}
	return reg.Register(consulLoaderDeletedTargets)
}
