When using Kafka as input, `gnmic` consumes data from a specific Kafka topic in `event` or `proto` format.

Multiple consumers can be created per `gnmic` instance (`num-workers`).
All the workers join the same [Kafka consumer group](https://docs.confluent.io/platform/current/clients/consumer.html#consumer-groups) (`group-id`) in order to load share the messages between the workers.

Multiple instances of `gnmic` with the same Kafka input can be used to effectively consume the exported messages in parallel

The Kafka input will export the received messages to the list of outputs configured under its `outputs` section.

```yaml
inputs:
  input1:
    # string, required, specifies the type of input
    type: kafka 
    # string, Kafka consumer name. 
    # If left empty, a name in `gnmic-$uuid` format is generated
    Name:
    # string, comma separated Kafka servers addresses
    address: localhost:9092
    # string, comma separated topics the Kafka consumer group consumes messages from.
    topics: telemetry 
    # consumer group all gnmic Kafka input workers join, 
    # so that Kafka server can load share the messages between them. Defaults to `gnmic-consumers`
    group-id: gnmic-consumers
    # duration, the timeout used to detect consumer failures when using Kafka's group management facility.
    # If no heartbeats are received by the broker before the expiration of this session timeout,
    # then the broker will remove this consumer from the group and initiate a rebalance.
    session-timeout: 10s
    # duration, the expected time between heartbeats to the consumer coordinator when using Kafka's group
	  # management facilities.
    heartbeat-interval: 3s
    # duration, wait time before reconnection attempts after any error
    recovery-wait-time: 2s 
    # string, kafka version, defaults to 2.5.0
    version: 
    # string, consumed message expected format, one of: proto, event
    format: event 
    # bool, enables extra logging
    debug: false
    # integer, number of kafka consumers to be created
    num-workers: 1
    # []string, list of named outputs to export data to. 
    # Must be configured under root level `outputs` section
    outputs: 
```

