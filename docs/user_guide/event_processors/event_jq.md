The `event-jq` processor, applies a [`jq`](https://stedolan.github.io/jq/) expression on the received event messages.

`jq` expressions are a powerful tool that can be used to slice, filter, map, transform JSON object.

The `event-jq` processor uses two configuration fields, `condition` and `expression`, both support `jq` expressions.

- `condition` (that needs to return a boolean value) determines if the processor is to be applied on the event message.
if `false` the message is returned as is.

- `expression` is used to transform, filter and/or enrich the messages. 
It needs to return a JSON object that can be mapped to an array of event messages.

The event messages resulting from a single `gNMI` Notification are passed to the jq expression as a JSON array.

Some `jq` expression examples:

- Select messages with name "sub1" that include a value called "counter1" with value higher than 90
```yaml
expression: .[] | select(.name=="sub1" and .values.counter1 > 90)
```

- Delete values with name "counter1"

```yaml
expression: .[] | del(.values.counter1)
```

- Delete values with names "counter1" or "counter2"

```yaml
expression: .[] | del(.values.["counter1", "counter2"])
```

- Delete tags with names "tag1" or "tag2"
```yaml
expression: .[] | del(.tags.["tag1", "tag2"])
```

- Add a tag called "my_new_tag" with value "tag1"
```yaml
expression: .[] |= (.tags.my_new_tag = "tag1")
```

- Move a value to tag under a custom key
```yaml
expression: .[] |= (.tags.my_new_tag_name = .values.value_name)
```

### Configuration

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-jq:
      # condition of application of the processor
      condition:
      # jq expression to transform/filter/enrich the message
      expression:
      # boolean enabling extra logging
      debug:
```
