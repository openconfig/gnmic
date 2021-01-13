`gnmic` supports exporting subscription updates to multiple Apache Kafka brokers/clusters simultaneously

A Kafka output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    type: kafka # required
    address: localhost:9092 # comma separated brokers addresses
    topic: telemetry # topic name
    max-retry: # max number of retries retry
    timeout: # kafka connection timeout
    recovery-wait-time: # wait time to reestablish the kafka producer connection after a failure
    format: # msg formatting, json, protojson, prototext, proto, event
    num-workers: # number of kafka producers to be created 
    debug: # (bool) enable debug
    buffer-size: # (int) number of messages to buffer before being picked up by the workers
    enable-metrics: false # boolean, enables the collection and export (via prometheus) of output specific metrics
    event-processors: # list of processors to apply on the mesage before writing
```

Currently all subscriptions updates (all targets and all subscriptions) are published to the defined topic name

When a Prometheus server is enabled, `gnmic` kafka output exposes 4 prometheus metrics, 3 Counters and 1 Gauge:

* `number_of_kafka_msgs_sent_success_total`: Number of msgs successfully sent by gnmic kafka output. This Counter is labeled with the kafka producerID
* `number_of_written_kafka_bytes_total`: Number of bytes written by gnmic kafka output. This Counter is labeled with the kafka producerID
* `number_of_kafka_msgs_sent_fail_total`: Number of failed msgs sent by gnmic kafka output. This Counter is labeled with the kafka producerID as well as the failure reason
* `msg_send_duration_ns`: gnmic kafka output send duration in nanoseconds. This Gauge is labeled with the kafka producerID