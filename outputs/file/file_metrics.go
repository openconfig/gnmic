package file

import "github.com/prometheus/client_golang/prometheus"

var numberOfWrittenBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_output",
	Name:      "number_bytes_written_total",
	Help:      "Number of bytes written to file output",
}, []string{"file_name"})

var numberOfReceivedMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_output",
	Name:      "number_messages_received_total",
	Help:      "Number of messages received by file output",
}, []string{"file_name"})

var numberOfWrittenMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_output",
	Name:      "number_messages_writes_total",
	Help:      "Number of messages written to file output",
}, []string{"file_name"})

var numberOfFailWriteMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_output",
	Name:      "number_messages_writes_fail_total",
	Help:      "Number of failed message writes to file output",
}, []string{"file_name", "reason"})

func initMetrics() {
	numberOfWrittenBytes.WithLabelValues("").Add(0)
	numberOfReceivedMsgs.WithLabelValues("").Add(0)
	numberOfWrittenMsgs.WithLabelValues("").Add(0)
	numberOfFailWriteMsgs.WithLabelValues("", "").Add(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(numberOfWrittenBytes); err != nil {
		return err
	}
	if err = reg.Register(numberOfReceivedMsgs); err != nil {
		return err
	}
	if err = reg.Register(numberOfWrittenMsgs); err != nil {
		return err
	}
	if err = reg.Register(numberOfFailWriteMsgs); err != nil {
		return err
	}
	return nil
}
