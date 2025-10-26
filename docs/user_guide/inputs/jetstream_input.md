When using jetstream as an input, `gnmic` consumes data from a specified NATS JetStream stream using a durable consumer. Messages are fetched in batches and delivered to `gnmic` in either `event` or `proto` format.

Multiple workers (subscribers) can be spawned using the `num-workers` option. Each worker independently connects to JetStream and fetches messages using the configured subject filters. All workers share the same durable consumer name to ensure coordinated message processing.

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
    subjects:
      - telemetry.device.*

    # enum string, consumer mode: "single" or "multi"
    # single: all workers share a single consumer (backward compatible, default)
    # multi: each worker creates its own filtered consumer (for workqueue streams)
    # default: "single"
    consumer-mode: single

    # list of subject filters for multi-consumer mode
    # ONLY used when consumer-mode is "multi"
    # each worker creates a consumer with these filter subjects
    filter-subjects:
      - telemetry.device.router1.*

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

## JetStream Consumer Modes

The JetStream input supports two consumer modes designed for different stream retention policies and processing patterns.

### Single Consumer Mode (Default)

In single consumer mode, all workers share the same durable consumer. This is the default and backward-compatible to gnmic prior to v0.42.

**Characteristics:**
- All workers connect to the same consumer using a shared durable name
- Messages are distributed across workers for parallel processing
- Works with both limits and workqueue retention policies
- Suitable for most use cases where you want to scale message consumption

**Example configuration:**

```yaml
inputs:
  shared-consumer:
    type: jetstream
    address: localhost:4222
    stream: telemetry-stream
    subjects:
      - telemetry.>
    consumer-mode: single  # default
    num-workers: 3
    format: event
```

### Multi Consumer Mode

In multi consumer mode, each gnmic instance creates its own filtered consumer. This mode is designed specifically for workqueue streams where you want different instances to process different subsets of messages based on subject filters.

**Characteristics:**
- Each gnmic instance creates a separate consumer with unique filter subjects
- Each consumer receives only messages matching its filter-subjects
- Requires workqueue retention policy on the stream
- Enables message routing based on subject patterns

**When to use multi consumer mode:**
- You have a workqueue stream with messages on different subjects
- You want different gnmic instances to process specific message subsets
- You need subject-based routing for parallel processing pipelines

**Example configuration:**

```yaml
inputs:
  filtered-consumer:
    type: jetstream
    address: localhost:4222
    stream: telemetry-stream
    consumer-mode: multi
    filter-subjects:
      - subscription1.x.>
      - subscription2.x.>
    num-workers: 2
    format: event
```

In this example, each of the 2 worker threads creates a consumer that receives only messages matching the filter subjects (subscription1.x.> or subscription2.x.>).

### Using Filter Subjects

The `filter-subjects` field is only used in multi consumer mode. It specifies which subjects each worker's consumer should filter for.

**Key points:**
- Only applies when `consumer-mode: multi`
- Supports NATS wildcard patterns (* and >)
- Each worker creates a consumer with the same filter subjects
- Messages matching any of the filter subjects will be delivered


### Choosing the Right Mode

**Use single consumer mode when:**
- You're using a limits retention stream for telemetry data
- You don't need multiple gnmic instances for scale out
- One gnmic instance can process all messages in the stream (limits or workqueue)
- You need backward compatibility with existing NATS+gnmic deployments.

**Use multi consumer mode when:**
- You're consuming from a workqueue stream and need scale out performance
- You need subject-based message routing
- Different gnmic instances should process different message subsets


## Usage Notes

- A durable consumer is created on the stream using the provided name as the durable name.

- All workers use the same durable name to share state and resume progress across reconnects.

- TLS can be configured if the NATS server uses secure connections.


