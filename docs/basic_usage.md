The following examples demonstrate the basic usage of the `gnmic` in a scenario where the remote target runs insecure (not TLS enabled) gNMI server. The `admin:admin` credentials are used to connect to the gNMI server running at `10.1.0.11:57400` address.

!!!info
    For the complete command usage examples, refer to the ["Command reference"](cmd/capabilities.md) menu.

### Capabilities RPC

Getting the device's [capabilities](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#32-capability-discovery) is done with [`capabilities`](cmd/capabilities.md) command:

```bash
gnmic -a 10.1.0.11:57400 -u admin -p admin --insecure capabilities
gNMI_Version: 0.7.0
supported models:
  - nokia-conf, Nokia, 19.10.R2
  - nokia-state, Nokia, 19.10.R2
  - nokia-li-state, Nokia, 19.10.R2
  - nokia-li-conf, Nokia, 19.10.R2
<< SNIPPED >>
supported encodings:
  - JSON
  - BYTES
```

### Get RPC

[Retrieving](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#33-retrieving-snapshots-of-state-information) the data snapshot from the target device is done with [`get`](cmd/get.md) command:

```bash
gnmic -a 10.1.0.11:57400 -u admin -p admin --insecure \
      get --path /state/system/platform

{
  "source": "10.1.0.11:57400",
  "timestamp": 1592829586901061761,
  "time": "2020-06-22T14:39:46.901061761+02:00",
  "updates": [
    {
      "Path": "state/system/platform",
      "values": {
        "state/system/platform": "7750 SR-1s"
      }
    }
  ]
}
```

### Set RPC

[Modifying](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#34-modifying-state) state of the target device is done with [`set`](cmd/set.md) command:

```bash
gnmic -a 10.1.0.11:57400 -u admin -p admin --insecure \
      set --update-path /configure/system/name \
          --update-value gnmic_demo

{
  "source": "0.tcp.eu.ngrok.io:12267",
  "timestamp": 1592831593821038738,
  "time": "2020-06-22T15:13:13.821038738+02:00",
  "results": [
    {
      "operation": "UPDATE",
      "path": "configure/system/name"
    }
  ]
}
```

### Subscribe RPC

[Subscription](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#35-subscribing-to-telemetry-updates) to the gNMI telemetry data can be done with [`subscribe`](cmd/subscribe.md) command:

```bash
gnmic -a 10.1.0.11:57400 -u admin -p admin --insecure \
      sub --path "/state/port[port-id=1/1/c1/1]/statistics/in-packets"

{
  "source": "0.tcp.eu.ngrok.io:12267",
  "timestamp": 1592832965197288856,
  "time": "2020-06-22T15:36:05.197288856+02:00",
  "prefix": "state/port[port-id=1/1/c1/1]/statistics",
  "updates": [
    {
      "Path": "in-packets",
      "values": {
        "in-packets": "12142"
      }
    }
  ]
}
```

### YANG path browser

`gnmic` can produce a list of XPATH/gNMI paths for a given YANG model with its [`path`](cmd/path.md) command. The paths in that list can be used as the `--path` values for the Get/Set/Subscribe commands.

```bash
# nokia model
gnmic path -m nokia-state --file nokia-state-combined.yang | head -10
/state/aaa/radius/statistics/coa/dropped/bad-authentication
/state/aaa/radius/statistics/coa/dropped/missing-auth-policy
/state/aaa/radius/statistics/coa/dropped/invalid
/state/aaa/radius/statistics/coa/dropped/missing-resource
/state/aaa/radius/statistics/coa/received
/state/aaa/radius/statistics/coa/accepted
/state/aaa/radius/statistics/coa/rejected
/state/aaa/radius/statistics/disconnect-messages/dropped/bad-authentication
/state/aaa/radius/statistics/disconnect-messages/dropped/missing-auth-policy
/state/aaa/radius/statistics/disconnect-messages/dropped/invalid
```
