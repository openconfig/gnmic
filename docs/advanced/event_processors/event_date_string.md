The `event_date_string` processor, converts a specific timestamp value (under tags or values) to a string representation. The format and location can be configured.

### Examples

```yaml
event_processors:
  # processor name
  convert_timestamp_processor:
    # processor type
    event_date_string:
      # list of regex to be matched with the values names
      value_names: 
        - "timestamp"
      timestamp_precision: ms
```
