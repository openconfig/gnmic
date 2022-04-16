The `event-duration-convert` processor, converts duration written as string to a integer with second precision.

The string format supported is a series of digits and a single letter indicating the unit, e.g 1w3d (1 week 3 days)
The highest unit is `w` for week and the lowest is `s` for second.
Any of the units may or may not be present.

### Examples

#### simple conversion

```yaml
processors:
  # processor name
  convert-uptime:
    # processor type
    event-duration-convert:
      # list of regex to be matched with the values names
      value-names: 
        - ".*_uptime$"
      # keep the original value, 
      # a new value name will be added with the converted value,
      # the new value name will be the original name with _seconds as suffix 
      keep: false
      # debug, enables this processor logging
      debug: false
```

=== "Event format before"
    ```json
    {
      "name": "default",
      "timestamp": 1607290633806716620,
      "tags": {
        "port_port-id": "A/1",
        "source": "172.17.0.100:57400",
        "subscription-name": "default"
      },
      "values": {
        "connection_uptime": "1w5s"
      }
    }
    ```
=== "Event format after"
    ```json
    {
      "name": "default",
      "timestamp": 1607290633806716620,
      "tags": {
        "port_port-id": "A/1",
        "source": "172.17.0.100:57400",
        "subscription-name": "default"
      },
      "values": {
        "connection_uptime": 604805
      }
    }
    ```
