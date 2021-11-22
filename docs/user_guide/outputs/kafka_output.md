`gnmic` supports exporting subscription updates to multiple Apache Kafka brokers/clusters simultaneously

### Configuration sample

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
    # Kafka TLS config
    tls:
      # path to certificate authority file, this will be used to verify the kafka server certificate
      ca-file:
      # path to client certificate file.
      cert-file: 
      # path to client key file.
      key-file:
      # boolean, controls whether a client verifies the server's certificate chain and host name
      # if set to true, the kafka client accepts any certificate presented by the server and any host name in that certificate
      skip-verify:
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
    # string, a GoTemplate that allows for the customization of the target field in Prefix.Target.
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
    # string, a GoTemplate that is executed using the received gNMI message as input.
    # the template execution is the last step before the data is written to the file,
    # First the received message is formatted according to the `format` field above, then the `event-processors` are applied if any
    # then finally the msg-template is executed.
    msg-template:
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

### Kafka Security protocol

Kafka clients can operate with 4 [security protocols](https://kafka.apache.org/24/javadoc/org/apache/kafka/common/security/auth/SecurityProtocol.html), 
their configuration is controlled via both `.tls` and `.sasl` fields under the output config.

**Security Protocol**  | **Description**                           | **Configuration**                       |
-----------------------|-------------------------------------------|-----------------------------------------|
`PLAINTEXT`            | Un-authenticated, non-encrypted channel   | `.tls` and `.sasl` are **NOT** present  |
`SASL_PLAINTEXT`       | SASL authenticated, non-encrypted channel | only `.sasl` is present                 |
`SASL_SSL`             | SASL authenticated, SSL channel           | both `.tls` and `.sasl` are present     |
`SSL`                  | SSL channel                               | only `.tls` is present                  |

#### Security Configuration Examples

=== "PLAINTEXT"
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        # other fields
        # no tls and no sasl fields
    ```
=== "SASL_PLAINTEXT"
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        sasl:
          user: admin
          password: secret
        # other fields
        # no tls field
    ```
=== "SASL_SSL"
    Example1: Without server certificate verification
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        sasl:
          user: admin
          password: secret
        tls:
          skip-verify: true
        # other fields
        # ...
    ```
    Example2: With server certificate verification
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        sasl:
          user: admin
          password: secret
        tls:
          ca-file: /path/to/ca-file
        # other fields
        # ...
    ```
    Example3: With client certificates
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        sasl:
          user: admin
          password: secret
        tls:
          cert-file: /path/to/cert-file
          key-file: /path/to/cert-file
        # other fields
        # ...
    ```
    Example4: With both server certificate verification and client certificates
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        sasl:
          user: admin
          password: secret
        tls:
          cert-file: /path/to/cert-file
          key-file: /path/to/cert-file
          ca-file: /path/to/ca-file
        # other fields
        # ...
    ```
=== "SSL"
    Example1: Without server certificate verification
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        tls:
          skip-verify: true
        # other fields
        # no sasl field
    ```
    Example2: With server certificate verification
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        tls:
          ca-file: /path/to/ca-file
        # other fields
        # no sasl field
    ```
    Example3: With client certificates
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        tls:
          cert-file: /path/to/cert-file
          key-file: /path/to/cert-file
        # other fields
        # no sasl field
    ```
    Example4: With both server certificate verification and client certificates
    ```yaml
    outputs:
      output1:
        type: kafka
        topic: my_kafka_topic
        tls:
          cert-file: /path/to/cert-file
          key-file: /path/to/cert-file
          ca-file: /path/to/ca-file
        # other fields
        # no sasl field
    ```

### Kafka Output Metrics

When a Prometheus server is enabled, `gnmic` kafka output exposes 4 prometheus metrics, 3 Counters and 1 Gauge:

* `number_of_kafka_msgs_sent_success_total`: Number of msgs successfully sent by gnmic kafka output. This Counter is labeled with the kafka producerID
* `number_of_written_kafka_bytes_total`: Number of bytes written by gnmic kafka output. This Counter is labeled with the kafka producerID
* `number_of_kafka_msgs_sent_fail_total`: Number of failed msgs sent by gnmic kafka output. This Counter is labeled with the kafka producerID as well as the failure reason
* `msg_send_duration_ns`: gnmic kafka output send duration in nanoseconds. This Gauge is labeled with the kafka producerID
