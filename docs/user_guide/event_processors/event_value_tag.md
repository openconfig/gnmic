The `event-value-tag` processor applies specific values from event messages to tags of other messages, if event tag names match.

Each [gNMI subscribe Response Update](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L95) in a [gNMI subscribe Response Notification](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L79) is transformed into an [Event Message](intro.md)  Additionally, if you are using an output cache, all [gNMI subscribe Response Update](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L95) messages are converted to Events on flush.

The `event-value-tag` processor is used to extract Values as tags to apply to other Events that have the same K:V tag pairs from the original event message, without merging events with different timestamps.

```yaml
processors:
  # processor name
  intf-description:
    # processor-type
    event-value-tag:
      # name of the value to match.  Usually a specific gNMI path
      value-name: "/interfaces/interface/state/description"
      # if set, use instead of the value name for tag
      tag-name: "description"
      # if true, remove value from original event when copying
      consume: false
      debug: false
```

=== "Event format before"
    ```json
    [
        {
            "name": "sub1",
            "timestamp": 1,
            "tags": {
                "source": "leaf1:6030",
                "subscription-name": "sub1",
                "interface_name": "Ethernet1"
            },
            "values": {
                "/interfaces/interface/status/counters/in-octets": 100
            }
        },
        {
            "name": "sub1",
            "timestamp": 200,
            "tags": {
                "source": "leaf1:6030",
                "subscription-name": "sub1",
                "interface_name": "Ethernet1"
            },
            "values": {
                "/interfaces/interface/status/counters/out-octets": 100
            }
        },
        {
            "name": "sub1",
            "timestamp": 200,
            "tags": {
                "source": "leaf1:6030",
                "subscription-name": "sub1",
                "interface_name": "Ethernet1"
            },
            "values": {
                "/interfaces/interface/status/description": "Uplink"
            }
        }
    ]
    ```
=== "Event format after"
    ```json
    [
        {
            "name": "sub1",
            "timestamp": 1,
            "tags": {
                "source": "leaf1:6030",
                "subscription-name": "sub1",
                "interface_name": "Ethernet1",
                "description": "Uplink"
            },
            "values": {
                "/interfaces/interface/status/counters/in-octets": 100
            }
        },
        {
            "name": "sub1",
            "timestamp": 200,
            "tags": {
                "source": "leaf1:6030",
                "subscription-name": "sub1",
                "interface_name": "Ethernet1",
                "description": "Uplink"
            },
            "values": {
                "/interfaces/interface/status/counters/out-octets": 100
            }
        },
        {
            "name": "sub1",
            "timestamp": 200,
            "tags": {
                "source": "leaf1:6030",
                "subscription-name": "sub1",
                "interface_name": "Ethernet1"
            },
            "values": {
                "/interfaces/interface/status/description": "Uplink"
            }
        }
    ]
    ```

```yaml
  bgp-description:
    event-value-tag:
      value-name: "neighbor_description"
      consume: true
```
=== "Event format before"
    ```json
    [
        {
            "name": "sub2",
            "timestamp": 1615284691523204299,
            "tags": {
                "neighbor_peer-address": "2002::1:1:1:1",
                "network-instance_name": "default",
                "source": "leaf1:57400",
                "subscription-name": "sub2"
            },
            "values": {
                "bgp_neighbor_sent_messages_queue_depth": 0,
                "bgp_neighbor_sent_messages_total_messages": "423",
                "bgp_neighbor_sent_messages_total_non_updates": "415",
                "bgp_neighbor_sent_messages_total_updates": "8"
            }
        },
        {
            "name": "sub2",
            "timestamp": 1615284691523204299,
            "tags": {
                "neighbor_peer-address": "2002::1:1:1:1",
                "network-instance_name": "default",
                "source": "leaf1:57400",
                "subscription-name": "sub2"
            },
            "values": {
                "neighbor_description": "PeerRouter"
            }
        }
    ]
    ```
=== "Event format after"
    ```json
    [
        {
            "name": "sub2",
            "timestamp": 1615284691523204299,
            "tags": {
                "neighbor_peer-address": "2002::1:1:1:1",
                "network-instance_name": "default",
                "source": "leaf1:57400",
                "subscription-name": "sub2"
                "neighbor_description": "PeerRouter"
            },
            "values": {
                "bgp_neighbor_sent_messages_queue_depth": 0,
                "bgp_neighbor_sent_messages_total_messages": "423",
                "bgp_neighbor_sent_messages_total_non_updates": "415",
                "bgp_neighbor_sent_messages_total_updates": "8",
            }
        },
        {
            "name": "sub2",
            "timestamp": 1615284691523204299,
            "tags": {
                "neighbor_peer-address": "2002::1:1:1:1",
                "network-instance_name": "default",
                "source": "leaf1:57400",
                "subscription-name": "sub2"
            },
            "values": {}
        }
    ]
    ```
