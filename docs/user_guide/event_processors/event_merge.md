The `event-merge` processor, merges multiple event messages together based on some criteria.

Each [gNMI subscribe Response Update](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L95) in a [gNMI subscribe Response Notification](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L79) is transformed into an [Event Message](intro.md)

The `event-merge` processor is used to merge the updates into one event message if it's needed.

The default merge strategy is based on the timestamp, the updates with the same timestamp will be merged into the same event message.

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-merge:
      # if always is set to true, 
      # the updates are merged regardless of the timestamp values
      always: false
      debug: false
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
