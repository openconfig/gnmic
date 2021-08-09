The `event-group-by` processor, groups values under the same event message based on a list of tag names.

This processor is intended to be used together with a output with cached gNMI notifications, like `prometheus` output with `gnmi-cache: true`.

### Configuration

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-group-by:
      # list of strings defining the tags to group by the values under 
      # a single event 
      tags: []
      # a boolean, if true only the values from events of the same name
      # are grouped together according to the list of tags
      by-name:
      # boolean
      debug: false
```

### Examples

#### group by a single tag

```yaml
processors:
  group-by-source:
    event-group-by:
      tags:
        - source
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
                "bgp_neighbor_received_messages_malformed_updates": "0",
                "bgp_neighbor_received_messages_queue_depth": 0,
                "bgp_neighbor_received_messages_total_messages": "424",
                "bgp_neighbor_received_messages_total_non_updates": "418",
                "bgp_neighbor_received_messages_total_updates": "6"
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
            },
            "values": {
                "bgp_neighbor_sent_messages_queue_depth": 0,
                "bgp_neighbor_sent_messages_total_messages": "423",
                "bgp_neighbor_sent_messages_total_non_updates": "415",
                "bgp_neighbor_sent_messages_total_updates": "8",
                "bgp_neighbor_received_messages_malformed_updates": "0",
                "bgp_neighbor_received_messages_queue_depth": 0,
                "bgp_neighbor_received_messages_total_messages": "424",
                "bgp_neighbor_received_messages_total_non_updates": "418",
                "bgp_neighbor_received_messages_total_updates": "6"
            }
        }
    ]
    ```

#### group by multiple tags

```yaml
processors:
  group-by-queue-id:
    event-group-by:
      tags:
        - source
        - interface_name
        - multicast-queue_queue-id
```

=== "Event Format Before"
    ```json
    [
      {
        "name": "sub1",
        "timestamp": 1627997491187771616,
        "tags": {
          "interface_name": "ethernet-1/1",
          "multicast-queue_queue-id": "5",
          "source": "clab-ndk-srl1:57400",
          "subscription-name": "sub1",
      },
        "values": {
          "/interface/qos/output/multicast-queue/queue-depth/maximum-burst-size": "0"
        }
      },
      {
        "name": "sub1",
        "timestamp": 1627997491187771616,
        "tags": {
          "interface_name": "ethernet-1/1",
          "multicast-queue_queue-id": "5",
          "source": "clab-ndk-srl1:57400",
          "subscription-name": "sub1",
        },
        "values": {
          "/interface/qos/output/multicast-queue/scheduling/peak-rate-bps": "0"
        }
      }
    ]
    ```
=== "Event Format After"
    ```json
    [
      {
        "name": "sub1",
        "timestamp": 1627997491187771616,
        "tags": {
          "interface_name": "ethernet-1/1",
          "multicast-queue_queue-id": "5",
          "source": "clab-ndk-srl1:57400",
          "subscription-name": "sub1",
      },
        "values": {
          "/interface/qos/output/multicast-queue/queue-depth/maximum-burst-size": "0",
          "/interface/qos/output/multicast-queue/scheduling/peak-rate-bps": "0"
        }
      }
    ]
    ```