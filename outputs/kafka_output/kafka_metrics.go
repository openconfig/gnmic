package kafka_output

import "github.com/prometheus/client_golang/prometheus"

var kafkaNumberOfSentMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "number_of_kafka_msgs_sent_success_total",
	Help:      "Number of msgs successfully sent by gnmic kafka output",
}, []string{"producer_id"})

var kafkaNumberOfSentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "number_of_written_kafka_bytes_total",
	Help:      "Number of bytes written by gnmic kafka output",
}, []string{"producer_id"})

var kafkaNumberOfFailSendMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "number_of_kafka_msgs_sent_fail_total",
	Help:      "Number of failed msgs sent by gnmic kafka output",
}, []string{"producer_id", "reason"})

var kafkaSendDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "kafka_output",
	Name:      "msg_send_duration_ns",
	Help:      "gnmic kafka output send duration in ns",
}, []string{"producer_id"})

func initMetrics() {
	kafkaNumberOfSentMsgs.WithLabelValues("").Add(0)
	kafkaNumberOfSentBytes.WithLabelValues("").Add(0)
	kafkaNumberOfFailSendMsgs.WithLabelValues("", "").Add(0)
	kafkaSendDuration.WithLabelValues("").Set(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(kafkaNumberOfSentMsgs); err != nil {
		return err
	}
	if err = reg.Register(kafkaNumberOfSentBytes); err != nil {
		return err
	}
	if err = reg.Register(kafkaNumberOfFailSendMsgs); err != nil {
		return err
	}
	if err = reg.Register(kafkaSendDuration); err != nil {
		return err
	}
	return nil
}
