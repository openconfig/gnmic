The `event-extract-tags` processor, extracts tags from a value, a value name, a tag name or a tag value using regex named groups.

It is possible to overwrite a tag if its name already exists.

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-extract-tags:
      # list of regular expressions to be used to extract strings to be added as a tag.
      tag-names:
      # list of regular expressions to be used to extract strings to be added as a tag.
      tags:
      # list of regular expressions to be used to extract strings to be added as a tag.
      value-names:
      # list of regular expressions to be used to extract strings to be added as a tag.
      values:
      # boolean, if true tags are over-written with the added ones if they already exist.
      overwrite:
      # boolean, enable extra logging
      debug:
```

### Examples

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-extract-tags:
      value-names:
        - `/(\w+)/(?P<group>\w+)/(\w+)`
```

=== "Event format before"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
            "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "3797",
            "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "288033",
            "/srl_nokia-interfaces:interface/statistics/in-octets": "65382630",
            "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "107154",
            "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "614",
            "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "11",
            "/srl_nokia-interfaces:interface/statistics/out-octets": "64721394",
            "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "105876"
        }
    }
    ```
=== "Event format after"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "group": "statistics",
            "subscription-name": "default"
        },
        "values": {
            "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
            "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "3797",
            "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "288033",
            "/srl_nokia-interfaces:interface/statistics/in-octets": "65382630",
            "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "107154",
            "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "614",
            "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "11",
            "/srl_nokia-interfaces:interface/statistics/out-octets": "64721394",
            "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "105876"
        }
    }
    ```