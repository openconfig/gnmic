The `event-add-tag` processor, adds a set of tags to an event message if one of the configured regular expressions in the values, value names, tags or tag names sections matches.

It is possible to overwrite a tag if it's name already exists.

### Examples

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-add-tag:
      value-names:
        - ".*-state$"
      add: 
        tag_name: tag_value
```

=== "Event format before"
    ```json
    //PLACEHOLDER
    ```
=== "Event format after"
    ```json
    //PLACEHOLDER
    ```
