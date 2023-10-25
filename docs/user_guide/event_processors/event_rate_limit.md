The `event-rate-limit` processor rate-limits each event with matching tags to the configured amount per-seconds.

All the tags for each event is hashed, and if the hash matches a previously seen event, then the timestamp 
of the event itself is compared to assess if the configured limit has been exceeded.
If it has, then this new event is dropped from the pipeline.

The cache for comparing timestamp is an LRU cache, with a default size of 1000 that can be increased for bigger deployments.

To account for cases where the device will artificially split the event into multiple chunks (with the same timestamp), 
the rate-limiter will ignore events with exactly the same timestamp.


### Examples

```yaml
processors:
  # processor name
  rate-limit-100pps:
    # processor type
    event-rate-limit:
      # rate of filtering, in events per seconds
      per-second: 100
      # set the cache size for doing the rate-limiting comparison
      # default value is 1000
      cache-size: 10000
      # debug for additionnal logging of dropped events
      debug: true
```