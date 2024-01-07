# Add hostname processor plugin

`event-add-hostname` is an event processor that gNMIc starts as a plugin. It enriches received gNMI notifications with the collector hostname as a tag.

## Build

To build the plugin run:

```bash
cd examples/plugins/event-add-hostname
go build -o event-add-hostname
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
  proc1:
    event-add-hostname:
      debug: true
      # the tag name to add with the host hostname as a tag value.
      hostname-tag-name: "collector-host"
      # read-interval controls how often the plugin runs the hostname cmd to get the host hostanme
      # by default it's at most every 1 minute
      read-interval: 1m
```

The processor type `event-add-hostname` should match the executable filename.

- Then add that processor under an output just like a you would do it with a regular processor:

```yaml
outputs:
  out1:
    type: file
    format: event
    event-processors:
      - proc1
```

The resulting event message should have a new tag called `collector-host`

```json
[
  {
    "name": "sub1",
    "timestamp": 1704572759243640092,
    "tags": {
      "collector-host": "kss",
      "interface_name": "ethernet-1/1",
      "source": "clab-ex-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/srl_nokia-interfaces:interface/statistics/out-octets": "4105346"
    }
  }
]
```
