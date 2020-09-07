Defining subscriptions with [`subscribe`](../cmd/subscribe.md) command's CLI flags is a quick&easy way to work with gNMI subscriptions. A downside of that approach though is that its not flexible enough.

With the multiple subscriptions defined in the [configuration file](file_cfg.md) we make a complex task of managing multiple subscriptions for multiple targets easy. The idea behind the multiple subscriptions is to define the subscriptions separately and then bind them to the targets.

### Defining subscriptions
To define a subscription a user needs to create the `subscriptions` container in the configuration file:

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

Each subscription is independent and fully configurable. The following list of configuration options is available:

* prefix
* target
* paths
* models
* mode
* stream-mode
* encoding
* qos
* sample-interval
* heartbeat-interval
* suppress-redundant
* updates-only

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
    If a target is not explicitely associated with any subscription, the client will subscribe to all defined subscriptions in the file.
    
The full configuration with the subscriptions defined and associated with targets will look like this:
```yaml
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
username: admin
password: nokiasr0s
insecure: true

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

```
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