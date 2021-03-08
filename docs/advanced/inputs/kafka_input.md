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
    # Kafka subscriber name
    # If left empty, it will be populated with the string from flag --instance-name appended with `--kafka-cons`.
    # If --instance-name is also empty, a random name is generated in the format `gnmic-$uuid`
    # note that each kafka worker (consumer) will get name=$name-$index
    name: ""
    # Kafka SASL configuration
    sasl:
      # SASL user name
      user:
      # SASL password
      password:
      # SASL mechanism: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512 and OAUTHBEARER are supported
      mechanism:
      # token url for OAUTHBEARER SASL mechanism
      token-url:
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
    # list of processors to apply on the message when received, 
    # only applies if format is 'event'
    event-processors: 
    # []string, list of named outputs to export data to. 
    # Must be configured under root level `outputs` section
    outputs: 
```

