`gnmic` supports exporting subscription updates to multiple NATS Streaming (STAN) servers/clusters simultaneously

A STAN output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    type: stan # required
    address: localhost:4223 # comma separated STAN servers
    subject: telemetry # stan subject
    subject-prefix: telemetry # stan subject prefix, the subject prefix is built the same way as for NATS output
    username: # STAN username
    password: # STAN password
    name: # client name
    cluster-name: test-cluster # cluster name
    ping-interval: # STAN ping interval
    ping-retry: # STAN ping retry
    format:  json # string, message marshaling format, one of: proto, prototext, protojson, json, event
    recovery-wait-time: 2s # duration to wait before re establishing a lost conneciton to a stan server
    num-workers: 1 # integer, number of nats publishers to be created
    debug: false # boolean, enables extra logging for the STAN output
    write-timeout: 10s # duration after which a message waiting to be handled by a worker gets discarded
    event-processors: # list of event-processors names to apply on the received messages
```

Using `subject` config value a user can specify the STAN subject to which to send all subscriptions updates for all targets

If a user wants to separate updates by targets and by subscriptions, `subject-prefix` can be used. if `subject-prefix` is specified `subject` is ignored.
