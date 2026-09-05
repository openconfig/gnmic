The `event-keep` processor removes tags or values that do not match the configured selectors. It complements `event-delete` and is useful when an event contains many fields but only a small allow-list is needed.

Selectors for names and values use OR semantics. Tags are filtered only when `tag-names` or `tags` is configured; values are filtered only when `value-names`, `value-name-paths`, or `values` is configured. An unconfigured category is left unchanged.

Events left without values, tags, or delete paths are discarded. Delete-only events are preserved.

`values` regular expressions match string values only. Retain numeric, boolean, structured,
or other non-string values with `value-names` or `value-name-paths`.

`value-name-paths` matches absolute, slash-separated paths without regular expressions. Literal segments match exactly and `*` matches one non-empty segment. Use it for large structured path allow-lists where regular expressions would be needlessly expensive.

```yaml
processors:
  keep-interface-counters:
    event-keep:
      value-names:
        - '^/system/uptime$'
      value-name-paths:
        - /interfaces/*/in-octets
        - /interfaces/*/out-octets
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
