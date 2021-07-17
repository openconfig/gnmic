package all

import (
	_ "github.com/karimra/gnmic/outputs/file"
	_ "github.com/karimra/gnmic/outputs/gnmi_output"
	_ "github.com/karimra/gnmic/outputs/influxdb_output"
	_ "github.com/karimra/gnmic/outputs/kafka_output"
	_ "github.com/karimra/gnmic/outputs/nats_output"
	_ "github.com/karimra/gnmic/outputs/prometheus_output"
	_ "github.com/karimra/gnmic/outputs/stan_output"
	_ "github.com/karimra/gnmic/outputs/tcp_output"
	_ "github.com/karimra/gnmic/outputs/udp_output"
)
