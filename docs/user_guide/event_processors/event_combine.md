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

### Conditional Execution of Subprocessors

The workflow for processing event messages can include multiple subprocessors, each potentially governed by its own condition. These conditions are defined using the jq query language, enabling dynamic and precise control over when each subprocessor should be executed.

### Defining Conditions for Subprocessors

When configuring your subprocessors, you have the option to attach a jq-based condition to each one. The specified condition acts as a gatekeeper, determining whether the corresponding subprocessor should be activated for a particular event message.

### Condition Evaluation Process

For a subprocessor to run, the following criteria must be met:

Condition Presence: If a condition is specified for the subprocessor, it must be evaluated.

Condition Outcome: The result of the jq condition evaluation must be true.

Combined Conditions: In scenarios where both the main processor and the subprocessor have associated conditions, both conditions must independently evaluate to true for the subprocessor to be triggered.

Only when all relevant conditions are met will the subprocessor execute its designated operations on the event message.

It is important to note that the absence of a condition is equivalent to a condition that always evaluates to true. Thus, if no condition is provided for a subprocessor, it will execute as long as the main processor's condition (if any) is met.

By using conditional execution, you can build sophisticated and efficient event message processing workflows that react dynamically to the content of the messages.

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
        - condition: '.tags["subscription-name"] == "sub2"'
          name: proc2
        - name: proc3
```
