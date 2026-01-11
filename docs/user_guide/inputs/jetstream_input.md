When using jetstream as an input, `gnmic` consumes data from a specified NATS JetStream stream using a durable consumer. Messages are fetched in batches and delivered to `gnmic` in either `event` or `proto` format.

Each gNMIc instance creates one durable consumer using the configured `subjects` field. Multiple workers (subscribers) can be spawned using the `num-workers` option to increase processing throughput. All workers within a single gNMIc instance share the same durable consumer to ensure coordinated message processing.

For scaling across multiple consumers, deploy multiple gNMIc instances with different consumer names. Each instance will create its own durable consumer on the stream.

The `jetstream` input will export received messages to the configured `outputs`. Optionally, `event-processors` can be applied when using event format.

```yaml
inputs:
  js-input:
    # required string, type of input
    type: jetstream

    # optional string, input instance name
    # defaults to a generated name if empty
    name: js-consumer

    # string, NATS server address
    # default: "localhost:4222"
    address: nats.example.com:4222

    # string, name of the JetStream stream to consume from
    stream: telemetry-stream

    # list of subject filters within the stream to consume from
    # the consumer will receive messages matching any of these subjects
    subjects:
      - telemetry.device.*

    # enum string, format of consumed message: "event" or "proto"
    # default: "event"
    format: event

    # enum string, delivery policy for JetStream:
    # one of: all, last, new, last-per-subject
    # default: all
    deliver-policy: last

    # optional string, subject format used to extract metadata
    # one of: static, subscription.target, target.subscription
    # affects proto messages only
    subject-format: target.subscription

    # optional string, NATS username
    username: nats-user

    # optional string, NATS password
    password: secret

    # optional duration, reconnect wait time
    # default: 2s
    connect-time-wait: 3s

    # optional bool, enables debug logging
    debug: true

    # integer, number of workers to start (parallel consumers)
    # default: 1
    num-workers: 2

    # integer, internal per-worker buffer size
    # default: 500
    buffer-size: 1000

    # integer, batch size when fetching messages from JetStream
    # default: 500
    fetch-batch-size: 200

    # integer, maximum number of allowed pending ack on the stream
    # default: 1000
    max-ack-pending: 5000

    # optional list of output names this input writes to
    # outputs must be configured at the root `outputs:` section
    outputs:
      - file
      - kafka

    # optional list of event processors
    # only applies when format is "event"
    event-processors:
      - add-tags

    # optional TLS configuration for secure NATS connection
    tls:
      ca-file: /etc/ssl/certs/ca.pem
      cert-file: /etc/ssl/certs/cert.pem
      key-file: /etc/ssl/certs/key.pem
      skip-verify: false
```

## Message Formats


- `event`: Expects JSON-encoded array of `EventMsg`. Supports processing pipelines and exporting to multiple outputs.

- `proto`: Expects binary-encoded `gnmi.SubscribeResponse` messages. Metadata such as `source` and `subscription-name` is extracted from the subject based on subject-format.

## Delivery Policies

- `all`: Delivers all messages from the stream history.

- `last`: Delivers only the most recent message.

- `new`: Starts delivery from new messages only.

- `last-per-subject`: Delivers the latest message for each subject.


## Subject Format Behavior

When using proto format, gnmic uses the subject name to extract metadata:

- `subscription.target` → subscription-name = first, source = second

- `target.subscription` → subscription-name = second, source = first

- `static` → no parsing; no additional metadata is extracted

## Scaling with Multiple Consumers

Each gNMIc instance creates a single durable consumer on the stream. To scale message processing across multiple consumers:

1. Deploy multiple gNMIc instances
2. Give each instance a different consumer `name` in its configuration
3. Configure each instance with appropriate `subjects` filters to partition work

**Example - Two instances consuming different subjects:**

Instance 1:
```yaml
inputs:
  js-consumer-1:
    type: jetstream
    name: consumer-router-metrics
    address: localhost:4222
    stream: telemetry-stream
    subjects:
      - telemetry.router.*
    num-workers: 2
    format: event
```

Instance 2:
```yaml
inputs:
  js-consumer-2:
    type: jetstream
    name: consumer-switch-metrics
    address: localhost:4222
    stream: telemetry-stream
    subjects:
      - telemetry.switch.*
    num-workers: 2
    format: event
```

Each instance creates its own durable consumer with its configured subject filters. Within each instance, multiple workers share the same consumer for parallel processing.

## Usage Notes

- A durable consumer is created on the stream using the provided name as the durable name.

- All workers use the same durable name to share state and resume progress across reconnects.

- TLS can be configured if the NATS server uses secure connections.


