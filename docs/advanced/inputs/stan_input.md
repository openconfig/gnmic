When using STAN as input, `gnmic` consumes data from a specific STAN subject in `event` or `proto` format.

Multiple consumers can be created per `gnmic` instance (`num-workers`).
All the workers join the same [STAN queue group](https://docs.stan.io/nats-concepts/queue) (`queue`) in order to load share the messages between the workers.

Multiple instances of `gnmic` with the same STAN input can be used to effectively consume the exported messages in parallel

The STAN input will export the received messages to the list of outputs configured under its `outputs` section.

```yaml
inputs:
  input1:
    # string, required, specifies the type of input
    type: stan 
    # STAN subscriber name
    # If left empty, it will be populated with the string from flag --instance-name appended with `--stan-sub`.
    # If --instance-name is also empty, a random name is generated in the format `gnmic-$uuid`
    # note that each stan worker (subscriber) will get name=$name-$index
    name: ""
    # string, comma separated STAN servers addresses
    address: localhost:4222
    # The subject name gnmic STAN consumers subscribe to.
    subject: telemetry 
    # subscribe queue group all gnmic STAN input workers join, 
    # so that STAN server can load share the messages between them.
    queue: 
    # string, STAN username
    username: 
    # string, STAN password  
    password: 
    # duration, wait time before reconnection attempts
    connect-time-wait: 2s
    # string, the STAN cluster name. defaults to test-cluster
    cluster-name: 
    # integer, interval (in seconds) at which 
    # a connection sends a PING to the server. min=1
    ping-interval:
    # integer, number of PINGs without a response 
    # before the connection is considered lost. min=2
    ping-retry:
    # string, consumed message expected format, one of: proto, event
    format: event 
    # bool, enables extra logging
    debug: false
    # integer, number of stan consumers to be created
    num-workers: 1
    # list of processors to apply on the message when received, 
    # only applies if format is 'event'
    event-processors: 
    # []string, list of named outputs to export data to. 
    # Must be configured under root level `outputs` section
    outputs: 
```

