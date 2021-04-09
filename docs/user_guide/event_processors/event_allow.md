The `event-allow` processor, allows only messages matching the configured `condition` or one of the regular expressions under `tags`, `tag-names`, `values` or `value-names`.

Non matching messages are dropped.

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-allow:
      # jq expression, if evaluated to true, the message is allowed
      condition: 
      # list of regular expressions to be matched against the tags names, 
      # if matched, the message is allowed
      tag-names:
      # list of regular expressions to be matched against the tags values,
      # if matched, the message is allowed
      tags:
      # list of regular expressions to be matched against the values names,
      # if matched, the message is allowed
      value-names:
      # list of regular expressions to be matched against the values,
      # if matched, the message is allowed
      values:
```
### Examples

```yaml
processors:
  # processor name
  allow-processor:
    # processor type
    event-allow:
      condition: ".tags.interface_name == 1/1/1"
```

=== "Event format before"
    ```json
    [
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
      },
      {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "1/1/1",
            "source": "172.23.23.3:57400",
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
    ]
    ```
=== "Event format after"
    ```json
    [
      {
      },
      {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "1/1/1",
            "source": "172.23.23.3:57400",
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
    ]
    ```


