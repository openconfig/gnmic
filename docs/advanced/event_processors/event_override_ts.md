The `event_override_ts` processor, overrides the message timestamp with `time.Now()`. The precision `s`, `ms`, `us` or `ns` (default) can be configured.

### Examples

```yaml
event_processors:
  # processor name
  set_timestamp_processor:
    # processor type
    event_override_ts:
      # timestamp precision, s, ms, us, ns (default)
      precision: ms
```
