The `event-date-string` processor, converts a specific timestamp value (under tags or values) to a string representation. The format and location can be configured.

### Examples

```yaml
processors:
  # processor name
  convert-timestamp-processor:
    # processor type
    event-date-string:
      # list of regex to be matched with the values names
      value-names: 
        - "timestamp"
      # received timestamp unit
      precision: ms
      # desired date string format, defaults to RFC3339
      format: "2006-01-02T15:04:05Z07:00"
      # timezone, defaults to the local timezone
      location: Asia/Taipei
```
