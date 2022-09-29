// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

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
