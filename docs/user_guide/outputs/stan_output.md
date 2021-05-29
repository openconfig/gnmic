`gnmic` supports exporting subscription updates to multiple NATS Streaming (STAN) servers/clusters simultaneously

A STAN output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    type: stan # required
    # comma separated STAN servers
    address: localhost:4222
    # stan subject
    subject: telemetry 
     # stan subject prefix, the subject prefix is built the same way as for NATS output
    subject-prefix: telemetry
    # STAN username
    username:
    # STAN password
    password: 
    # STAN publisher name
    # if left empty, this field is populated with the output name used as output ID (output1 in this example).
    # the full name will be '$(name)-stan-pub'.
    # If the flag --instance-name is not empty, the full name will be '$(instance-name)-$(name)-stan-pub.
    # note that each stan worker (publisher) will get client name=$name-$index
    name: ""
    # cluster name, mandatory
    cluster-name: test-cluster
    # STAN ping interval
    ping-interval: 5
    # STAN ping retry
    ping-retry: 2
    # string, message marshaling format, one of: proto, prototext, protojson, json, event
    format:  event 
    # boolean, if true the message timestamp is changed to current time
    override-timestamps: false
    # duration to wait before re establishing a lost connection to a stan server
    recovery-wait-time: 2s
    # integer, number of stan publishers to be created
    num-workers: 1 
    # boolean, enables extra logging for the STAN output
    debug: false 
    # duration after which a message waiting to be handled by a worker gets discarded
    write-timeout: 10s 
    # boolean, enables the collection and export (via prometheus) of output specific metrics
    enable-metrics: false 
    # list of processors to apply on the message before writing
    event-processors: 
```

Using `subject` config value a user can specify the STAN subject to which to send all subscriptions updates for all targets

If a user wants to separate updates by targets and by subscriptions, `subject-prefix` can be used. if `subject-prefix` is specified `subject` is ignored.
