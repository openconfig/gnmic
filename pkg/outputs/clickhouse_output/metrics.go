// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package clickhouse_output

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerMetricsOnce sync.Once

var chReceivedRows = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "clickhouse_output",
	Name:      "received_rows_total",
	Help:      "Rows received by the ClickHouse output (after event split)",
}, []string{"name", "table"})

var chInsertedRows = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "clickhouse_output",
	Name:      "inserted_rows_total",
	Help:      "Rows successfully inserted into ClickHouse",
}, []string{"name", "table"})

var chFailedRows = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "clickhouse_output",
	Name:      "failed_rows_total",
	Help:      "Rows lost due to insert failures after retries",
}, []string{"name", "table", "reason"})

var chDroppedRows = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "clickhouse_output",
	Name:      "dropped_rows_total",
	Help:      "Rows dropped (e.g. buffer full / enqueue timeout)",
}, []string{"name", "table", "reason"})

var chInsertBatches = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "clickhouse_output",
	Name:      "insert_batches_total",
	Help:      "Number of batch insert operations",
}, []string{"name", "table"})

var chInsertBatchSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "gnmic",
	Subsystem: "clickhouse_output",
	Name:      "insert_batch_size",
	Help:      "Distribution of insert batch sizes",
	Buckets:   prometheus.ExponentialBuckets(1, 2, 16),
}, []string{"name", "table"})

var chInsertDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "gnmic",
	Subsystem: "clickhouse_output",
	Name:      "insert_duration_seconds",
	Help:      "ClickHouse batch insert duration",
	Buckets:   prometheus.DefBuckets,
}, []string{"name", "table"})

var chBufferLen = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "clickhouse_output",
	Name:      "buffer_len",
	Help:      "Approximate number of rows waiting in the output buffer channel",
}, []string{"name", "table"})

func (o *clickhouseOutput) initMetricLabels() {
	cfg := o.cfg.Load()
	if cfg == nil || !cfg.EnableMetrics {
		return
	}
	t := cfg.Table
	chReceivedRows.WithLabelValues(cfg.Name, t).Add(0)
	chInsertedRows.WithLabelValues(cfg.Name, t).Add(0)
	chFailedRows.WithLabelValues(cfg.Name, t, "shutdown").Add(0)
	chFailedRows.WithLabelValues(cfg.Name, t, "no_connection").Add(0)
	chDroppedRows.WithLabelValues(cfg.Name, t, "enqueue_timeout").Add(0)
	chInsertBatches.WithLabelValues(cfg.Name, t).Add(0)
	chInsertBatchSize.WithLabelValues(cfg.Name, t).Observe(0)
	chInsertDuration.WithLabelValues(cfg.Name, t).Observe(0)
	chBufferLen.WithLabelValues(cfg.Name, t).Set(0)
}

func (o *clickhouseOutput) registerMetrics() error {
	cfg := o.cfg.Load()
	if cfg == nil || !cfg.EnableMetrics {
		return nil
	}
	if o.reg == nil {
		o.logger.Error("metrics enabled but main registry is not initialized, enable main metrics under `api-server`")
		return nil
	}
	var err error
	registerMetricsOnce.Do(func() {
		collectors := []prometheus.Collector{
			chReceivedRows, chInsertedRows, chFailedRows, chDroppedRows,
			chInsertBatches, chInsertBatchSize, chInsertDuration, chBufferLen,
		}
		for _, c := range collectors {
			if err = o.reg.Register(c); err != nil {
				return
			}
		}
	})
	return err
}
