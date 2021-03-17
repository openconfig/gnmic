The `event-trigger` processor, triggers an action if the configured expression evaluates to `true`.

The expression is evaluated using the [expr](https://github.com/antonmedv/expr) package with the event message as input.

Examples:

- The below expression checks if the value named `counter1` exists and has a value higher than 90
```bash
#     condition      ?          yes              : no
"counter1" in Values ? (Values["counter1"] > 90) : false
```

- This expression checks if the event name is `sub1` and that the tag `source` exists and is equal to `r1:57400`
```bash
Name == "sub1" and (("source" in Tags ? (Tags["source"] == "r1:57400")): false)
```

The trigger can be monitored over a configurable window of time (default 1 minute), during which only a certain number of occurrences (default 1) trigger the action.

Actions can be of two types:

- HTTP action: trigger an HTTP request
- gNMI action: trigger a Get or Set gnmi RPC

### HTTP Action

Using the HTTP action you can send the whole event message to a remote server, 
or apply some transformation on the message before sending it.

Transforming the message can be done in 2 different ways, either with an [expr](https://github.com/antonmedv/expr) expression 
or using Go templates.

the below
```yaml
processors:
  my_trigger_proc:
    event-trigger:
      expression: 
      max-occurrences:
      window:
      action:
        type: http
        url: http://remote-server:p8080/
```
### gNMI Action

```yaml
```

### Examples
