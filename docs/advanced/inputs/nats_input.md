When using NATS as input, `gnmic` consumes data from a specific NATS subject in `event` or `proto` format.

Multiple consumers can be created per `gnmic` instance (`num-workers`).
All the workers join the same [NATS queue group](https://docs.nats.io/nats-concepts/queue) (`queue`) in order to load share the messages between the workers.

Multiple instances of `gnmic` with the same NATS input can be used to effectively consume the exported messages in parallel

The NATS input will export the received messages to the list of outputs configured under its `outputs` section.

```yaml
inputs:
  input1:
    # string, required, specifies the type of input
    type: nats 
    # string, NATS consumer name. 
    # If left empty, a name in `gnmic-$uuid` format is generated
    Name:
    # string, comma separated NATS servers addresses
    address: localhost:4222
    # The subject name gnmic NATS consumers subscribe to.
    subject: telemetry 
    # subscribe queue group all gnmic NATS input workers join, 
    # so that NATS server can load share the messages between them.
    queue: 
    # string, NATS username
    username: 
    # string, NATS password  
    password: 
    # duration, wait time before reconnection attempts
    connect-time-wait: 2s 
    # string, consumed message expected format, one of: proto, event
    format: event 
    # bool, enables extra logging
    debug: false
    # integer, number of nats consumers to be created
    num-workers: 1
    # integer, sets the size of the local buffer where received 
    # NATS messages are stored before being sent to outputs.
    # This value is set per worker. Defaults to 100 messages
    buffer-size: 100
    # []string, list of named outputs to export data to. 
    # Must be configured under root level `outputs` section
    outputs: 
```

