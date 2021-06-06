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
