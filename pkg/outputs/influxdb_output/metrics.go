// SPDX-License-Identifier: Apache-2.0

package influxdb_output

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerMetricsOnce sync.Once

// influxNonFiniteDropped counts float fields dropped as NaN/±Inf, which line
// protocol cannot encode. Otherwise the drops are silent.
var influxNonFiniteDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "influxdb_output",
	Name:      "non_finite_values_dropped_total",
	Help:      "Float fields dropped because their value was NaN or +/-Inf and cannot be encoded in line protocol",
}, []string{"name", "measurement"})

func (i *influxDBOutput) initMetricLabels() {
	cfg := i.cfg.Load()
	if cfg == nil || !cfg.EnableMetrics {
		return
	}
	influxNonFiniteDropped.WithLabelValues(cfg.Name, "default").Add(0)
}

func (i *influxDBOutput) registerMetrics() error {
	cfg := i.cfg.Load()
	if cfg == nil || !cfg.EnableMetrics {
		return nil
	}
	if i.reg == nil {
		i.logger.Error("metrics enabled but main registry is not initialized, enable main metrics under `api-server`")
		return nil
	}
	var err error
	registerMetricsOnce.Do(func() {
		for _, c := range []prometheus.Collector{influxNonFiniteDropped} {
			if err = i.reg.Register(c); err != nil {
				return
			}
		}
	})
	return err
}
