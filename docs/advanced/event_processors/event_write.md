The `event_write` processor,  writes a message that has a value or a tag matching one of the configured regular expressions to `stdout`, `stderr` or to a file. 
A custom separator (used between written messages) can be configured, it defaults to `\n`

### Examples
```yaml
event_processors:
  # processor name
  delete_processor:
    # processor type
    event_write:
      value_names:
        - ".*-state"
      dst: log-file.log
      separator: "\n####\n"
      indent: "  "
```
