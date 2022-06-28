package all

import (
	_ "github.com/karimra/gnmic/outputs/file"
	_ "github.com/karimra/gnmic/outputs/gnmi_output"
	_ "github.com/karimra/gnmic/outputs/influxdb_output"
	_ "github.com/karimra/gnmic/outputs/kafka_output"
	_ "github.com/karimra/gnmic/outputs/nats_outputs/jetstream"
	_ "github.com/karimra/gnmic/outputs/nats_outputs/nats"
	_ "github.com/karimra/gnmic/outputs/nats_outputs/stan"
	_ "github.com/karimra/gnmic/outputs/prometheus_output/prometheus_output"
	_ "github.com/karimra/gnmic/outputs/prometheus_output/prometheus_write_output"
	_ "github.com/karimra/gnmic/outputs/tcp_output"
	_ "github.com/karimra/gnmic/outputs/udp_output"
)
