package file

import "github.com/prometheus/client_golang/prometheus"

var NumberOfWrittenBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_output",
	Name:      "number_bytes_written_total",
	Help:      "Number of bytes written to file output",
}, []string{"file_name"})

var NumberOfReceivedMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_output",
	Name:      "number_messages_received_total",
	Help:      "Number of messages received by file output",
}, []string{"file_name"})

var NumberOfWrittenMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_output",
	Name:      "number_messages_writes_total",
	Help:      "Number of messages written to file output",
}, []string{"file_name"})

var NumberOfFailWriteMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "file_output",
	Name:      "number_messages_writes_fail_total",
	Help:      "Number of failed message writes to file output",
}, []string{"file_name", "reason"})

func initMetrics() {
	NumberOfWrittenBytes.WithLabelValues("").Add(0)
	NumberOfReceivedMsgs.WithLabelValues("").Add(0)
	NumberOfWrittenMsgs.WithLabelValues("").Add(0)
	NumberOfFailWriteMsgs.WithLabelValues("", "").Add(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(NumberOfWrittenBytes); err != nil {
		return err
	}
	if err = reg.Register(NumberOfReceivedMsgs); err != nil {
		return err
	}
	if err = reg.Register(NumberOfWrittenMsgs); err != nil {
		return err
	}
	if err = reg.Register(NumberOfFailWriteMsgs); err != nil {
		return err
	}
	return nil
}
