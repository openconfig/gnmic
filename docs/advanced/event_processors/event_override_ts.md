The `event-override-ts` processor, overrides the message timestamp with `time.Now()`. The precision `s`, `ms`, `us` or `ns` (default) can be configured.

### Examples

```yaml
processors:
  # processor name
  set-timestamp-processor:
    # processor type
    event-override-ts:
      # timestamp precision, s, ms, us, ns (default)
      precision: ms
```
