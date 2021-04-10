The `event-add-tag` processor, adds a set of tags to an event message if one of the configured regular expressions in the values, value names, tags or tag names sections matches.

It is possible to overwrite a tag if it's name already exists.

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-add-tag:
      # jq expression, if evaluated to true, the tags are added
      condition: 
      # list of regular expressions to be matched against the tags names, if matched, the tags are added
      tag-names:
      # list of regular expressions to be matched against the tags values, if matched, the tags are added
      tags:
      # list of regular expressions to be matched against the values names, if matched, the tags are added
      value-names:
      # list of regular expressions to be matched against the values, if matched, the tags are added
      values:
      # boolean, if true tags are over-written with the added ones if they already exist.
      overwrite:
      # map of tags to be added
      add: 
        tag_name: tag_value
```

### Examples

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-add-tag:
      value-names:
        - "."
      add: 
        tag_name: tag_value
```

=== "Event format before"
    ```json
    {
      "name": "sub1",
      "timestamp": 1607678293684962443,
      "tags": {
        "interface_name": "mgmt0",
        "source": "172.20.20.5:57400"
      },
      "values": {
        "Carrier_Transitions": 1,
        "In_Broadcast_Packets": 448,
        "In_Error_Packets": 0,
        "In_Fcs_Error_Packets": 0,
        "In_Multicast_Packets": 47578,
        "In_Octets": 15557349,
        "In_Unicast_Packets": 6482,
        "Out_Broadcast_Packets": 110,
        "Out_Error_Packets": 0,
        "Out_Multicast_Packets": 10,
        "Out_Octets": 464766
      }
    }
    ```
=== "Event format after"
    ```json
    {
      "name": "sub1",
      "timestamp": 1607678293684962443,
      "tags": {
        "interface_name": "mgmt0",
        "source": "172.20.20.5:57400",
        "tag_name": "tag_value"
    },
      "values": {
        "Carrier_Transitions": 1,
        "In_Broadcast_Packets": 448,
        "In_Error_Packets": 0,
        "In_Fcs_Error_Packets": 0,
        "In_Multicast_Packets": 47578,
        "In_Octets": 15557349,
        "In_Unicast_Packets": 6482,
        "Out_Broadcast_Packets": 110,
        "Out_Error_Packets": 0,
        "Out_Multicast_Packets": 10,
        "Out_Octets": 464766
      }
    }
    ```
