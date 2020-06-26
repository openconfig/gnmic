package all

import (
	_ "github.com/karimra/gnmiClient/outputs/file"
	_ "github.com/karimra/gnmiClient/outputs/kafka_output"
	_ "github.com/karimra/gnmiClient/outputs/nats_output"
	_ "github.com/karimra/gnmiClient/outputs/stan_output"
)
