The `event-ieeefloat32` processor allows converting binary data received from a router with the type IEEE 32-bit floating point number.


```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-ieeefloat32:
      # jq expression, if evaluated to true, the processor applies based on the field `value-names`
      condition: 
      # list of regular expressions to be matched against the values names, if matched, the value is converted to a float32.
      value-names: []
```

### Examples

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-ieeefloat32:
      value-names:
        - "^components/component/power-supply/state/output-current$"
```

=== "Event format before"
    ```json
    {
      "name": "sub1",
      "timestamp": 1607678293684962443,
      "tags": {
        "source": "172.20.20.5:57400"
      },
      "values": {
        "components/component/power-supply/state/output-current": "QEYAAA=="
      }
    }
    ```
=== "Event format after"
    ```json
    {
      "name": "sub1",
      "timestamp": 1607678293684962443,
      "tags": {
        "source": "172.20.20.5:57400",
    },
      "values": {
        "components/component/power-supply/state/output-current": 3.09375
      }
    }
    ```
