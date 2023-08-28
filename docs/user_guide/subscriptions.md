
Defining subscriptions with [`subscribe`](../cmd/subscribe.md) command's CLI flags is a quick&easy way to work with gNMI subscriptions. 

A downside of that approach is that commands can get lengthy when defining multiple subscriptions and not all possible flavors and combinations of subscription can be defined.

With the multiple subscriptions defined in the [configuration file](configuration_file.md) we make a complex task of managing multiple subscriptions for multiple targets easy. The idea behind the multiple subscriptions is to define the subscriptions separately and then bind them to the targets.

## Defining subscriptions

### CLI-based subscription

A subscription is configured through a series of command-line interface (CLI) flags. These include, *but are not limited to*:

1. `--path`: This flag is used to set the paths for the subscription.

2. `--mode [once | poll | stream]`: Defines the subscription mode. It can be set to once, poll, or stream.

3. `--stream-mode [target-defined | sample | on-change]`: Sets the stream subscription mode. The options are target-defined, sample, or on-change.

4. `--sample-interval`: Determines the sample interval for a stream/sample subscription.

A command executed with these flags will generate a single [SubscribeRequest](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3511-the-subscriberequest-message) that is sent to the target.

Every path configured with the `--path` flag leads to a [`Subscription`](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3513-the-subscription-message) added to the [`subscriptionList`](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3512-the-subscriptionlist-message) message.

There are no constraints when defining a `ONCE` or `POLL` subscribe request. However, when a `STREAM` subscribe request is defined using flags, all subscriptions (paths) will adopt the same mode (`target-defined`, `on-change`, or `sample`) and stream subscription attributes such as `sample-interval` and `heartbeat-interval`.

### File-based subscription config

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
    # list of strings, the list of outputs to send updates to. If blank, defaults to all outputs
    outputs:
      - output1
      - output2
    # list of subscription definition, this field is used to define multiple stream subscriptions (target-defined, sample or on-change)
    # that will be created using a single SubscribeRequest (i.e: share the same gRPC stream).
    # This field cannot be defined if `paths`, `stream-mode`, `sample-interval`, `heartbeat-interval` or`suppress-redundant` are set.
    # Only fields applicable to STREAM subscriptions can be set in this list of subscriptions: 
    # `paths`, `stream-mode`, `sample-interval`, `heartbeat-interval` or`suppress-redundant`
    stream-subscriptions:
      - paths: []
        stream-mode: 
        sample-interval:
        heartbeat-interval:
        suppress-redundant:
      - paths: []
        stream-mode: 
        sample-interval:
        heartbeat-interval:
        suppress-redundant:
    # historical subscription config: https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-history.md#1-purpose    
    history:
      # string, nanoseconds since Unix epoch or RFC3339 format.
      # if set, the history extension type will be a Snapshot request
      snapshot:
      # string, nanoseconds since Unix epoch or RFC3339 format.
      # if set, the history extension type will be a Range request
      start:
      # string, nanoseconds since Unix epoch or RFC3339 format.
      # if set, the history extension type will be a Range request
      end:
```

#### Subscription config to gNMI SubscribeRequest

Each subscription (under `subscriptions:`) results in a single [`SubscribeRequest`](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3511-the-subscriberequest-message) being sent to the target.

If `paths` is set, each path results in a separate [`Subscription`](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3513-the-subscription-message) message being added to the [`subscriptionList`](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3512-the-subscriptionlist-message) message.

If instead of paths, a list of stream-subscriptions is defined:

```yaml
subscriptions:
  sub1:
    stream-subscriptions:
      - paths:
```

Each path under each stream-subscriptions will result in a separate [`Subscription`](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3513-the-subscription-message) message being added to the [`subscriptionList`](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#3512-the-subscriptionlist-message) message.

#### Examples

##### A single stream/sample subscription

=== "YAML"
    ```yaml
    subscriptions:
      port_stats:
        paths:
          - "/state/port[port-id=*]/statistics"
        stream-mode: sample
        sample-interval: 5s
        encoding: bytes
    ```
=== "CLI"
    ```shell
    gnmic sub --path /state/port/statistics \
              --stream-mode sample \
              --sample-interval 5s \
              --encoding bytes
    ```
=== "PROTOTEXT"
    ```text
    subscribe: {
      subscription: {
        path: {
          elem: {
            name: "state"
          }
          elem: {
            name: "port"
          }
          elem: {
            name: "statistics"
          }
        }
        mode: SAMPLE
        sample_interval:  5000000000
      }
      encoding: BYTES
    }
    ```

##### A single stream/on-change subscription

=== "YAML"
    ```yaml
    subscriptions:
      port_stats:
        paths:
          - "/state/port/oper-state"
        stream-mode: on-change
        encoding: bytes
    ```
=== "CLI"
    ```shell
    gnmic sub --path /state/port/oper-state \
              --stream-mode on-change \
              --encoding bytes
    ```
=== "PROTOTEXT"
    ```text
    subscribe: {
      subscription: {
        path: {
          elem: {
            name: "state"
          }
          elem: {
            name: "port"
          }
          elem: {
            name: "oper-state"
          }
        }
        mode: ON_CHANGE
      }
      encoding: BYTES
    }
    ```

##### A ONCE subscription

=== "YAML"
    ```yaml
    subscriptions:
      system_facts:
        paths:
          - /configure/system/name
          - /state/system/version
        mode: once
        encoding: bytes
    ```
=== "CLI"
    ```shell
    gnmic sub --path /configure/system/name \
              --path /state/system/version \
              --mode once \
              --encoding bytes
    ```
=== "PROTOTEXT"
    ```text
    subscribe: {
      subscription: {
        path: {
          elem: {
            name: "configure"
          }
          elem: {
            name: "port"
          }
          elem: {
            name: "name"
          }
        }
      }
      subscription: {
        path: {
          elem: {
            name: "state"
          }
          elem: {
            name: "system"
          }
          elem: {
            name: "version"
          }
        }
      }
      mode: ONCE
      encoding: BYTES
    }
    ```

##### Combining multiple stream subscriptions in the same gRPC stream

=== "YAML"
    ```yaml
    subscriptions:
      sub1:
        stream-subscriptions:
          - paths:
            - /configure/system/name
            stream-mode: on-change
          - paths:
            - /state/port/statistics
            stream-mode: sample
            sample-interval: 10s  
        encoding: bytes
    ```
=== "CLI"
    NA
=== "PROTOTEXT"
    ```text
    subscribe: {
      subscription: {
        path: {
          elem: {
            name: "configure"
          }
          elem: {
            name: "system"
          }
          elem: {
            name: "name"
          }
        }
        mode: ON_CHANGE
      }
      subscription: {
        path: {
          elem: {
            name: "state"
          }
          elem: {
            name: "port"
          }
          elem: {
            name: "statistics"
          }
        }
        mode: SAMPLE
        sample_interval:  10000000000
      }
      encoding: BYTES
    }
    ```

##### Configure multiple subscriptions

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

## Binding subscriptions

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
