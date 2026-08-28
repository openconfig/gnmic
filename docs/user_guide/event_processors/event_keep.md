The `event-keep` processor removes tags or values that do not match the configured regular expressions. It complements `event-delete` and is useful when an event contains many fields but only a small allow-list is needed.

Selectors for names and values use OR semantics. Tags are filtered only when `tag-names` or `tags` is configured; values are filtered only when `value-names` or `values` is configured. An unconfigured category is left unchanged.

```yaml
processors:
  keep-interface-counters:
    event-keep:
      value-names:
        - '^/interfaces/[^/]+/(in-octets|out-octets)$'
      tag-names:
        - '^(resource_id|source)$'
```

Given the following event:

```json
{
  "tags": {
    "resource_id": "leaf-1",
    "source": "192.0.2.1:57400",
    "subscription-name": "interfaces"
  },
  "values": {
    "/interfaces/ethernet-1/in-octets": 42,
    "/interfaces/ethernet-1/out-octets": 24,
    "/vendor/debug": "ignored"
  }
}
```

the processor keeps the two interface counters and the `resource_id` and `source` tags.
