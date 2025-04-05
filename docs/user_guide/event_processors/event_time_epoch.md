The event-time-epoch processor is a plugin for gNMIc that converts string-based time values in event messages into epoch timestamps.
This is particularly useful when input data includes timestamps in human-readable formats (like RFC3339) and you want to normalize them for downstream systems.

# Configuration

```yaml
processors:
  convert-timestamp:
    event-time-epoch:
      value-names:
        - ".*timestamp"
        - "lastSeen"
      format: "2006-01-02T15:04:05Z07:00"
      precision: "ms"
      debug: true
```

| Field         | Type       | Description                                                                                      |
|---------------|------------|--------------------------------------------------------------------------------------------------|
| `value-names` | `[]string` | List of regular expressions to match against the event `values` keys. Only matching keys will be processed. |
| `format`      | `string`   | [Go time layout](https://golang.org/pkg/time/#Time.Format) used to parse the input strings. Defaults to RFC3339 format. |
| `precision`   | `string`   | Desired epoch output precision: `s`, `ms`, `us`, or `ns`. Defaults to nanoseconds.              |
| `debug`       | `bool`     | Enables verbose logging to stderr or the provided logger.                                        |


=== "Event format before"
    ```json
    {
        "name": "default",
        "timestamp": 1607290633806716620,
        "tags": {
            "source": "172.17.0.100:57400",
            "subscription-name": "default"
        },
        "values": {
            "system/timestamp": "2025-04-05T10:30:00Z"
        }
    }
    ```
=== "Event format after"
    ```json
    {
        "name": "default",
        "timestamp": 1607290633806716620,
        "tags": {
            "source": "172.17.0.100:57400",
            "subscription-name": "default"
        },
        "values": {
            "system.timestamp": 1743849000
        }
    }
    ```
