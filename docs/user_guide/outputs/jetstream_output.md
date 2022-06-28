`gnmic` supports exporting subscription updates [NATS Jetstream](https://docs.nats.io/nats-concepts/jetstream) servers.

A [Jetstream](https://docs.nats.io/nats-concepts/jetstream) output can be defined using the below format in `gnmic` config file under `outputs` section:

### configuration

```yaml
outputs:
  output1:
    # required
    type: jetstream 
    # NATS publisher name
    # if left empty, this field is populated with the output name used as output ID (output1 in this example).
    # If the flag --instance-name is not empty, the full name will be '$(instance-name)-$(name).
    # note that each jetstream worker (publisher) will get a client name=$name-$index
    name: ""
    # Comma separated NATS servers
    address: localhost:4222
    # string, stream name to write update to,
    # if `create-stream` is set, it will be created
    # # may not contain spaces, tabs, period (.), greater than (>) or asterisk (*)
    stream: 
    # defines stream parameters that gNMIc will create on the target jetstream server(s)
    create-stream:
      # string, stream description
      description: created by gNMIc
      # string list, list of subjects allowed on the stream
      # defaults to `.create-stream.$name.>`
      subjects:
      # string, one of `memory`, `file`.
      # defines the storage type to use for the stream.
      # defaults to `memory`
      storage:
      # int64, max number of messages in the stream.
      max-msgs:
      # int64, max bytes the stream may contain.
      max-bytes:
      # duration, max age of any message in the stream.
      max-age:
      # int32, maximum message size
      max-msg-size:
    # string, one of `static`, `subscription.target`, `subscription.target.path` 
    # or `subscription.target.pathKeys`.
    # Defines the subject format.
    # `static`: 
    #       all updates will be written to the subject name set under `outputs.$output_name.subject`
    # `subscription.target`: 
    #       updates from each subscription, target will be written 
    #       to subject $subscription_name.$target_name
    # `subscription.target.path`: 
    #       updates from a certain subscription, target and path 
    #       will be written to subject $subscription_name.$target_name.$path.
    #       The path is built by joining the gNMI path pathElements with a dot (.).
    #       e.g: /interface[name=ethernet-1/1]/statistics/in-octets
    #       -->  interface.statistics.in-octets 
    # `subscription.target.pathKeys`: 
    #       updates from a certain subscription, a certain target and a certain path 
    #       will be written to subject $subscription_name.$target_name.$path.
    #       The path is built by joining the gNMI path pathElements and Keys with a dot (.).
    #       e.g: /interface[name=ethernet-1/1]/statistics/in-octets
    #       -->  interface.{name=ethernet-1/1}.statistics.in-octets 
    subject-format: static 
    # If a subject-format is `static`, gnmic will publish all subscriptions updates 
    # to a single subject configured under this field. Defaults to 'telemetry'
    subject: telemetry
    # TLS configuration
    tls:
      # string, path to CA certificates file
      ca-file:
      # string, path to client certificate file
      cert-file:
      # string, path to client key file
      key-file:
      # boolean, if true, the client does not verify the server certificates
      skip-verify:
    # NATS username
    username: 
    # NATS password  
    password: 
    # wait time before reconnection attempts
    connect-time-wait: 2s 
    # Exported message format, one of: proto, prototext, protojson, json, event
    format: event 
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
    # the template execution is the last step before the data is written to the file.
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
    # list of processors to apply to the message before writing
    event-processors: 
```

### subject-format

The `subject-format` field is used to control how the received gNMI notifications are written into the configured stream.

#### static

All notifications will be written to the subject name set under `outputs.$output_name.subject`

#### subscription.target

Notifications from each subscription and target pair will be written to subject `$subscription_name.$target_name`

#### subscription.target.path

Notifications from a subscription, target and path tuple
will be written to subject $subscription_name.$target_name.$path.
The path is built by joining the gNMI path pathElements with a period `(.)`.

Notifications containing more than one update, will be expanded into multiple notifications with one update each.

E.g:

An update from target `target1` and subscription `sub1` containing path `/interface[name=ethernet-1/1]/statistics/in-octets`,
will be written to subject:

```text
$stream_name.sub1.target1.interface.statistics.in-octets
```

#### subscription.target.pathKeys

Updates from a certain subscription, a certain target and a certain path will be written to subject `$subscription_name.$target_name.$path`.
The path is built by joining the gNMI path pathElements and Keys with a period `(.)`.

Notifications containing more than one update, will be expanded into multiple notifications with one update each.

E.g:

An update from target `target1` and subscription `sub1` containing path `/interface[name=ethernet-1/1]/statistics/in-octets`,
will be written to subject:

```text
$stream_name.sub1.target1.interface.{name=ethernet-1/1}.statistics.in-octets
```
