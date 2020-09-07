`gnmic` supports exporting subscription updates to multiple NATS servers/clusters simultaneously

A [NATS](https://docs.nats.io/) output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  group1:
    - type: nats # required
      address: localhost:4222 # comma separated NATS servers
      subject-prefix: telemetry # this prefix is used to to build the subject name for each target/subscription
      subject: telemetry # if a subject-prefix is not specified, gnmic will publish all subscriptions updates to a single subject 'telemetry'
      username: # NATS username
      password: # NATS password  
      connect-time-wait: # wait time before reconnection attempts
```

Using `subject` config value a user can specify the NATS subject to which to send all subscriptions updates for all targets

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
