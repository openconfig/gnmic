package file

import "github.com/prometheus/client_golang/prometheus"

var NumberOfWrittenBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "number_of_written_bytes_total",
	Help: "Number of bytes written to file",
}, []string{"file_name"})
var NumberOfReceivedMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "number_of_received_messages_total",
	Help: "Number of messages received by file output",
}, []string{"file_name"})
var NumberOfWrittenMsgs = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "number_of_written_messages_total",
	Help: "Number of messages written to file",
}, []string{"file_name"})
