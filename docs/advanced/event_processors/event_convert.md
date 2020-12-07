The `event_convert` processor, converts the values matching one of the regular expressions to a specific type: `uint`, `int`, `string`, `float`

### Examples

```yaml
event_processors:
  # processor name
  convert_int_processor:
    # processor type
    event_convert:
      # list of regex to be matched with the values names
      value_names: 
        - ".*octets$"
      # the desired value type, one of int, uint, string, float
      type: int 
```

=== "before"
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
        "/state/port/ethernet/statistics/in-octets": "7753940"
      }
    }
    ```
=== "after"
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
        "/state/port/ethernet/statistics/in-octets": 7753940
      }
    }
    ```