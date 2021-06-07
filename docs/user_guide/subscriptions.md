

Defining subscriptions with [`subscribe`](../cmd/subscribe.md) command's CLI flags is a quick&easy way to work with gNMI subscriptions. A downside of that approach is that commands can get lengthy when defining multiple subscriptions.

With the multiple subscriptions defined in the [configuration file](configuration_file.md) we make a complex task of managing multiple subscriptions for multiple targets easy. The idea behind the multiple subscriptions is to define the subscriptions separately and then bind them to the targets.

### Defining subscriptions

To define a subscription a user needs to create the `subscriptions` container in the configuration file:

```yaml
subscriptions:
  # a configurable subscription name
  subscription-name:
    # string, path to be set as the Subscribe Request Prefix
    prefix:
    # string, value to set as the SubscribeRequest Prefix Target
    target:
    # boolean, if true, the SubscribeRequest Prefix Target will be set to 
    # the configured target name under section `targets`.
    # does not apply if the previous field `target` is set.
    set-target: # true | false
    # list of strings, list of subscription paths for the named subscription
    paths: []
    # list of strings, schema definition modules
    models: []
    # string, case insensitive, one of ONCE, STREAM, POLL
    mode: STREAM
    # string, case insensitive, if `mode` is set to STREAM, this defines the type 
    # of streamed subscription,
    # one of SAMPLE, TARGET_DEFINED, ON_CHANGE
    stream-mode: TARGET_DEFINED
    # string, case insensitive, defines the gNMI encoding to be used for the subscription
    encoding: JSON
    # integer, specifies the packet marking that is to be used for the subscribe responses
    qos:
    # duration, Golang duration format, e.g: 1s, 1m30s, 1h.
    # specifies the sample interval for a STREAM/SAMPLE subscription
    sample-interval:
    # duration, Golang duration format, e.g: 1s, 1m30s, 1h.
    # The heartbeat interval value can be specified along with `ON_CHANGE` or `SAMPLE` 
    # stream subscriptions modes and has the following meanings in each case:
    # - `ON_CHANGE`: The value of the data item(s) MUST be re-sent once per heartbeat 
    #                interval regardless of whether the value has changed or not.
    # - `SAMPLE`: The target MUST generate one telemetry update per heartbeat interval, 
    #             regardless of whether the `--suppress-redundant` flag is set to true.
    heartbeat-interval:
    # boolean, if set to true, the target SHOULD NOT generate a telemetry update message unless 
    # the value of the path being reported on has changed since the last 
    suppress-redundant:
    # boolean, if set to true, the target MUST not transmit the current state of the paths 
    # that the client has subscribed to, but rather should send only updates to them.
    updates-only:
```

Examples:

```yaml
# part of ~/gnmic.yml config file
subscriptions:  # container for subscriptions
  port_stats:     # a named subscription, a key is a name
    paths:      # list of subscription paths for that named subscription
      - "/state/port[port-id=1/1/c1/1]/statistics/out-octets"
      - "/state/port[port-id=1/1/c1/1]/statistics/in-octets"
    stream-mode: sample # one of [on-change target-defined sample]
    sample-interval: 5s
    encoding: bytes
  service_state:
    paths:
       - "/state/service/vpls[service-name=*]/oper-state"
       - "/state/service/vprn[service-name=*]/oper-state"
    stream-mode: on-change
  system_facts:
    paths:
       - "/configure/system/name"
       - "/state/system/version"
    mode: once
```

Inside that subscriptions container a user defines individual named subscriptions; in the example above two named subscriptions `port_stats` and `service_state` were defined.

These subscriptions can be used on the cli via the `[ --name ]` flag of subscribe command:

```shell
gnmic subscribe --name service_state --name port_stats
```

Or by binding them to different targets, (see next section)

### Binding subscriptions

Once the subscriptions are defined, they can be flexibly associated with the targets.

```yaml
# part of ~/gnmic.yml config file
targets:
  router1.lab.com:
    username: admin
    password: secret
    subscriptions:
      - port_stats
      - service_state
  router2.lab.com:
    username: gnmi
    password: telemetry
    subscriptions:
      - service_state
```

The named subscriptions are put under the `subscriptions` section of a target container. As shown in the example above, it is allowed to add multiple named subscriptions under a single target; in that case each named subscription will result in a separate Subscription Request towards a target.

!!! note
    If a target is not explicitly associated with any subscription, the client will subscribe to all defined subscriptions in the file.

The full configuration with the subscriptions defined and associated with targets will look like this:

```yaml
username: admin
password: nokiasr0s
insecure: true

targets:
  router1.lab.com:
    subscriptions:
      - port_stats
      - service_state
      - system_facts
  router2.lab.com:
    subscriptions:
      - service_state
      - system_facts

subscriptions:
  port_stats:
    paths:
      - "/state/port[port-id=1/1/c1/1]/statistics/out-octets"
      - "/state/port[port-id=1/1/c1/1]/statistics/in-octets"
    stream-mode: sample
    sample-interval: 5s
    encoding: bytes
  service_state:
    paths:
       - "/state/service/vpls[service-name=*]/oper-state"
       - "/state/service/vprn[service-name=*]/oper-state"
    stream-mode: on-change
  system_facts:
    paths:
       - "/configure/system/name"
       - "/state/system/version"
    mode: once
```

As a result of such configuration the `gnmic` will set up three gNMI subscriptions to router1 and two other gNMI subscriptions to router2:

```shell
$ gnmic subscribe
gnmic 2020/07/06 22:03:35.579942 target 'router2.lab.com' initialized
gnmic 2020/07/06 22:03:35.593082 target 'router1.lab.com' initialized
```

```json
{
  "source": "router2.lab.com",
  "subscription-name": "service_state",
  "timestamp": 1594065869313065895,
  "time": "2020-07-06T22:04:29.313065895+02:00",
  "prefix": "state/service/vpls[service-name=testvpls]",
  "updates": [
    {
      "Path": "oper-state",
      "values": {
        "oper-state": "down"
      }
    }
  ]
}
{
  "source": "router1.lab.com",
  "subscription-name": "service_state",
  "timestamp": 1594065868850351364,
  "time": "2020-07-06T22:04:28.850351364+02:00",
  "prefix": "state/service/vpls[service-name=test]",
  "updates": [
    {
      "Path": "oper-state",
      "values": {
        "oper-state": "down"
      }
    }
  ]
}
{
  "source": "router1.lab.com",
  "subscription-name": "port_stats",
  "timestamp": 1594065873938155916,
  "time": "2020-07-06T22:04:33.938155916+02:00",
  "prefix": "state/port[port-id=1/1/c1/1]/statistics",
  "updates": [
    {
      "Path": "in-octets",
      "values": {
        "in-octets": "671552"
      }
    }
  ]
}
{
  "source": "router1.lab.com",
  "subscription-name": "port_stats",
  "timestamp": 1594065873938043848,
  "time": "2020-07-06T22:04:33.938043848+02:00",
  "prefix": "state/port[port-id=1/1/c1/1]/statistics",
  "updates": [
    {
      "Path": "out-octets",
      "values": {
        "out-octets": "370930"
      }
    }
  ]
}
^C
received signal 'interrupt'. terminating...
```
