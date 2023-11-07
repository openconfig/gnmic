The `event-combine` processor combines multiple processors together. 
This allows to declare processors once and reuse them to build more complex processors.

### Configuration

```yaml
processors:
  # processor name
  pipeline1:
    # processor type
    event-combine:
      # list of regex to be matched with the values names
      processors: 
          # The "sub" processor execution condition. A jq expression.
        - condition: 
          # the processor name, should be declared in the
          # `processors` section.
          name: 
      # enable extra logging
      debug: false
```

### Examples

In the below example, we define 3 regular processors and 2 `event-combine` processors.

- `proc1`: Allows event message that have tag `"interface_name = ethernet-1/1`

- `proc2`: Renames values names to their path base.
             e.g: `interface/statistics/out-octets` --> `out-octets`

- `proc3`: Converts any values with a name ending with `octets` to `int`.

- `pipeline1`: Combines `proc1`, `proc2` and `proc3`, applying `proc2` only to subscription `sub1`

- `pipeline2`: Combines `proc2` and `proc3`, applying `proc2` only to subscription `sub2`

The 2 combine processors can be linked with different outputs.

```yaml
processors:
  proc1:
    event-allow:
      condition: '.tags.interface_name == "ethernet-1/1"'

  proc2:
    event-strings:
      value-names:
        - ".*"
      transforms:
        - path-base:
            apply-on: "name"
  proc3:
    event-convert:
      value-names: 
        - ".*octets$"
      type: int 
  

  pipeline1:
    event-combine:
      processors: 
        - name: proc1
        - condition: '.tags["subscription-name"] == "sub1"'
          name: proc2
        - name: proc3
  
  pipeline2:
    event-combine:
      processors: 
        - condition: '.tags[\"subscription-name\"] == \"sub2\"'
          name: proc2
        - name: proc3
```
