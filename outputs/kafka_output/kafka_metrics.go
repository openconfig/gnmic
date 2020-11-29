package kafka_output

import "github.com/prometheus/client_golang/prometheus"

var KafkaNumberOfSentMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "number_of_written_kafka_msgs_total",
	Help:      "Number of msgs written by gnmic kafka output",
}, []string{"producer_id"})

var KafkaNumberOfSentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "number_of_written_kafka_bytes_total",
	Help:      "Number of bytes written by gnmic kafka output",
}, []string{"producer_id"})

var KafkaNumberOfFailSendMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "number_of_failed_send_kafka_msgs_total",
	Help:      "Number of failed msgs to be sent by gnmic kafka output",
}, []string{"producer_id", "reason"})

var KafkaSendDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "write_duration_ns",
	Help:      "gnmic kafka output send duration in ns",
}, []string{"producer_id"})

func initMetrics() {
	KafkaNumberOfSentMsgs.WithLabelValues("").Add(0)
	KafkaNumberOfSentBytes.WithLabelValues("").Add(0)
	KafkaNumberOfFailSendMsgs.WithLabelValues("", "").Add(0)
	KafkaSendDuration.WithLabelValues("").Set(0)
}
