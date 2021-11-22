`gnmic` supports exporting subscription updates to multiple NATS servers/clusters simultaneously

A [NATS](https://docs.nats.io/) output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    # required
    type: nats 
    # NATS publisher name
    # if left empty, this field is populated with the output name used as output ID (output1 in this example).
    # the full name will be '$(name)-nats-pub'.
    # If the flag --instance-name is not empty, the full name will be '$(instance-name)-$(name)-nats-pub.
    # note that each nats worker (publisher) will get client name=$name-$index
    name: ""
    # Comma separated NATS servers
    address: localhost:4222 
    # This prefix is used to to build the subject name for each target/subscription
    subject-prefix: telemetry 
    # If a subject-prefix is not specified, gnmic will publish all subscriptions updates to a single subject configured under this field. Defaults to 'telemetry'
    subject: telemetry 
    # NATS username
    username: 
    # NATS password  
    password: 
    # wait time before reconnection attempts
    connect-time-wait: 2s 
    # Exported message format, one of: proto, prototext, protojson, json, event
    format: json 
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
    # string, a GoTemplate that is executed using the received gNMI message as input.
    # the template execution is the last step before the data is written to the file,
    # First the received message is formatted according to the `format` field above, then the `event-processors` are applied if any
    # then finally the msg-template is executed.
    msg-template:
    # boolean, if true the message timestamp is changed to current time
    override-timestamps: false
    # integer, number of nats publishers to be created
    num-workers: 1 
    # duration after which a message waiting to be handled by a worker gets discarded
    write-timeout: 5s 
    # boolean, enables extra logging for the nats output
    debug: false
    # boolean, enables the collection and export (via prometheus) of output specific metrics
    enable-metrics: false 
    # list of processors to apply on the message before writing
    event-processors: 
```

Using `subject` config value, a user can specify the NATS subject to which to send all subscriptions updates for all targets

If a user wants to separate updates by targets and by subscriptions, `subject-prefix` can be used. if `subject-prefix` is specified `subject` is ignored.

`gnmic` takes advantage of NATS [subject hierarchy](https://docs.nats.io/nats-concepts/subjects#subject-hierarchies) by publishing gNMI subscription updates to a separate subject per target per subscription.

The NATS subject name is built out of the `subject-prefix`, `name` under the target definition and `subscription-name` resulting in the following format: `subject-prefix.name.subscription-name`

e.g: for a target `router1`, a subscription name `port-stats` and subject-prefix `telemetry` the subject name will be `telemetry.router1.port-stats`

If the target name is an IP address, or a hostname (meaning potentially contains `.`), the `.` characters are replaced with a `-`

e.g: for a target `172.17.0.100:57400`, the previous subject name becomes `telemetry.172-17-0-100:57400.port-stats`

This way a user can subscribe to different subsets of updates by tweaking the subject name:

* `"telemetry.>"` gets all updates sent to NATS by all targets, all subscriptions
* `"telemetry.router1.>"` gets all NATS updates for target router1
* `"telemetry.*.port-stats"` gets all updates from subscription port-stats, for all targets
