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
    timeout: # connection timeout
    ping-interval: # STAN ping interval
    ping-retry: # STAN ping retry
```

Using `subject` config value a user can specify the STAN subject to which to send all subscriptions updates for all targets

If a user wants to separate updates by targets and by subscriptions, `subject-prefix` can be used. if `subject-prefix` is specified `subject` is ignored.
