The `event-to-tag` processor, moves a value matching one of the regular expressions from the values section to the tags section.
It's possible to keep the value under values section after moving it.

### Examples

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-to-tag:
      value-names:
        - ".*-state$"
```

=== "Event format before"
    ```json
    {
        "name": "default",
        "timestamp": 1607305284170936330,
        "tags": {
            "interface_name": "ethernet-1/1",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "/srl_nokia-interfaces:interface/admin-state": "disable",
            "/srl_nokia-interfaces:interface/ifindex": 54,
            "/srl_nokia-interfaces:interface/last-change": "2020-11-20T05:52:21.459Z",
            "/srl_nokia-interfaces:interface/oper-down-reason": "port-admin-disabled",
            "/srl_nokia-interfaces:interface/oper-state": "down"
        }
    }
    ```
=== "Event format after"
    ```json
    {
        "name": "default",
        "timestamp": 1607305284170936330,
        "tags": {
            "interface_name": "ethernet-1/1",
            "source": "172.23.23.2:57400",
            "subscription-name": "default",
            "/srl_nokia-interfaces:interface/admin-state": "disable",
            "/srl_nokia-interfaces:interface/oper-state": "down"
        },
        "values": {
            "/srl_nokia-interfaces:interface/ifindex": 54,
            "/srl_nokia-interfaces:interface/last-change": "2020-11-20T05:52:21.459Z",
            "/srl_nokia-interfaces:interface/oper-down-reason": "port-admin-disabled"
        }
    }
    ```
