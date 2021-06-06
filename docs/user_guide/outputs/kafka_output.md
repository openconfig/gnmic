`gnmic` supports exporting subscription updates to multiple Apache Kafka brokers/clusters simultaneously

A Kafka output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    # required
    type: kafka 
    # kafka client name. 
    # if left empty, this field is populated with the output name used as output ID (output1 in this example).
    # the full name will be '$(name)-kafka-prod'.
    # If the flag --instance-name is not empty, the full name will be '$(instance-name)-$(name)-kafka-prod.
    # note that each kafka worker (producer) will get client name=$name-$index
    name: ""
    # Comma separated brokers addresses
    address: localhost:9092 
    # Kafka topic name
    topic: telemetry 
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
    # The total number of times to retry sending a message
    max-retry: 2 
    # Kafka connection timeout
    timeout: 5s 
    # Wait time to reestablish the kafka producer connection after a failure
    recovery-wait-time: 10s 
    # Exported msg format, json, protojson, prototext, proto, event
    format: event 
    # string, one of `overwrite`, `if-not-present`, ``
    # This field allows populating/changing the value of Prefix.Target in the received message.
    # if set to ``, nothing changes 
    # if set to `overwrite`, the target value is overwritten using the template configured under `target-template`
    # if set to `if-not-present`, the target value is populated only if it is empty, still using the `target-template`
    add-target: 
    # string, a GoTemplate that allow for the customization of the target field in Prefix.Target.
    # it applies only if the previous field `add-target` is not empty.
    # if left empty, it defaults to:
    # {{- if index . "subscription-target" -}}
    # {{ index . "subscription-target" }}
    # {{- else -}}
    # {{ index . "source" | host }}
    # {{- end -}}`
    # which will set the target to the value configured under `subscription.$subscription-name.target` if any,
    # otherwise it will set it to the target name stripped of the port number (if present)
    target-template:
    # boolean, if true the message timestamp is changed to current time
    override-timestamps: false
    # Number of kafka producers to be created 
    num-workers: 1 
    # (bool) enable debug
    debug: false 
    # (int) number of messages to buffer before being picked up by the workers
    buffer-size: 0 
    # (bool) enables the collection and export (via prometheus) of output specific metrics
    enable-metrics: false 
    # list of processors to apply on the message before writing
    event-processors: 
```

Currently all subscriptions updates (all targets and all subscriptions) are published to the defined topic name

When a Prometheus server is enabled, `gnmic` kafka output exposes 4 prometheus metrics, 3 Counters and 1 Gauge:

* `number_of_kafka_msgs_sent_success_total`: Number of msgs successfully sent by gnmic kafka output. This Counter is labeled with the kafka producerID
* `number_of_written_kafka_bytes_total`: Number of bytes written by gnmic kafka output. This Counter is labeled with the kafka producerID
* `number_of_kafka_msgs_sent_fail_total`: Number of failed msgs sent by gnmic kafka output. This Counter is labeled with the kafka producerID as well as the failure reason
* `msg_send_duration_ns`: gnmic kafka output send duration in nanoseconds. This Gauge is labeled with the kafka producerID