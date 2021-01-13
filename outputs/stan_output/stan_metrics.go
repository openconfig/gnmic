package stan_output

import "github.com/prometheus/client_golang/prometheus"

var StanNumberOfSentMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "stan_output",
	Name:      "number_of_stan_msgs_sent_success_total",
	Help:      "Number of msgs successfully sent by gnmic stan output",
}, []string{"publisher_id", "subject"})

var StanNumberOfSentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "stan_output",
	Name:      "number_of_written_stan_bytes_total",
	Help:      "Number of bytes written by gnmic stan output",
}, []string{"publisher_id", "subject"})

var StanNumberOfFailSendMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "gnmic",
	Subsystem: "stan_output",
	Name:      "number_of_stan_msgs_sent_fail_total",
	Help:      "Number of failed msgs sent by gnmic stan output",
}, []string{"publisher_id", "reason"})

var StanSendDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "gnmic",
	Subsystem: "stan_output",
	Name:      "msg_send_duration_ns",
	Help:      "gnmic stan output send duration in ns",
}, []string{"publisher_id"})

func initMetrics() {
	StanNumberOfSentMsgs.WithLabelValues("", "").Add(0)
	StanNumberOfSentBytes.WithLabelValues("", "").Add(0)
	StanNumberOfFailSendMsgs.WithLabelValues("", "").Add(0)
	StanSendDuration.WithLabelValues("").Set(0)
}

func registerMetrics(reg *prometheus.Registry) error {
	initMetrics()
	var err error
	if err = reg.Register(StanNumberOfSentMsgs); err != nil {
		return err
	}
	if err = reg.Register(StanNumberOfSentBytes); err != nil {
		return err
	}
	if err = reg.Register(StanNumberOfFailSendMsgs); err != nil {
		return err
	}
	if err = reg.Register(StanSendDuration); err != nil {
		return err
	}
	return nil
}
