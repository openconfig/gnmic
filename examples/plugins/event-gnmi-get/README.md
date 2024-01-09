# gNMI Get based notification enriching processor plugin

`event-gnmi-get` is an event processor that gNMIc starts as a plugin. It enriches received gNMI notifications with tags retrieved using a gNMI Get RPC.

## Building the plugin

```bash
cd examples/plugins/event-gnmi-get
go build -o event-gnmi-get
```

## Running the plugin

- To run the plugin point gNMIc to the directory where the plugin binary resides. Either using the flag `--plugin-processors-path | -P`:

```bash
gnmic --config gnmic.yaml subscribe -P /path/to/plugin/bin
```

Or using the config file:

```yaml
plugins:
  path: /path/to/plugin/bin
  glob: "*"
  start-timeout: 0s
```

This allows gNMIc to discover the plugin executable and initialize it. Make sure the files gNMIc loads are executable.

- Next configure the plugin as a processor:

```yaml
processors:
  proc2:
    event-gnmi-get:
      debug: true
      encoding: ascii
      data-type: all
      paths:
        - path: "platform/chassis/type"
          tag-name: "chassis-type"
        - path: "platform/chassis/hw-mac-address"
          tag-name: "hw-mac-address"
        - path: "system/name/host-name"
          tag-name: "hostname"
```

The processor type `event-gnmi-get` should match the executable filename.

- Then add that processor under an output just like a you would do it with a regular processor:

```yaml
outputs:
  out1:
    type: file
    format: event
    event-processors:
      - proc2
```

The resulting event message should have a set of new tags called `chassis-type`, `hw-mac-address` and `hostname`.

```json
[
  {
    "name": "sub1",
    "timestamp": 1704573345190497607,
    "tags": {
      "chassis-type": "7220 IXR-D2",
      "hostname": "srl1",
      "interface_name": "ethernet-1/1",
      "hw-mac-address": "1A:F2:00:FF:00:00",
      "source": "clab-ex-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/srl_nokia-interfaces:interface/statistics/out-octets": "4108666"
    }
  }
]
```

## Examples

### trigger get request directly to the node

```yaml
processors:
  proc1:
    event-gnmi-get:
      debug: true
      encoding: ascii
      data-type: all
      paths:
        - path: "platform/chassis/type"
          tag-name: "chassis-type"
        - path: "platform/chassis/hw-mac-address"
          tag-name: "hw-mac-address"
        - path: "system/name/host-name"
          tag-name: "hostname"
```

### trigger get request through gNMIc's gNMI server

```yaml
# enable gNMIc gNMI server
gnmi-server:
  address: :57401

processors:
  proc1:
    event-gnmi-get:
      debug: true
      encoding: ascii
      data-type: all
      # set the gNMI Get target to the local gNMI server address
      target: localhost:57401
      # include the actual target name in the GetRequest Prefix
      prefix-target: '{{ index .Tags "source" }}'
      paths:
        - path: "platform/chassis/type"
          tag-name: "chassis-type"
        - path: "platform/chassis/hw-mac-address"
          tag-name: "hw-mac-address"
        - path: "system/name/host-name"
          tag-name: "hostname"
```
