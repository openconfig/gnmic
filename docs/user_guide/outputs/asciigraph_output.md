`gnmic` supports displaying collected metrics as an ASCII graph on the terminal.
The graph is generated using the [asciigraph](https://github.com/guptarohit/asciigraph) package.

### Configuration sample

```yaml

outputs:
  output1:
    # required
    type: asciigraph
    # string, the graph caption
    caption: 
    # integer, the graph height. If unset, defaults to the terminal height
    height:
    # integer, the graph width. If unset, defaults to the terminal width
    width:
    # float, the graph minimum value for the vertical axis.
    lower-bound:
    # float, the graph minimum value for the vertical axis.
    upper-bound:
    # integer, the graph left offset.
    offset:
    # integer, the decimal point precision of the label values.
    precision:
    # string, the caption color. one of ANSI colors.
    caption-color:
    # string, the axis color. one of ANSI colors.
    axis-color:
    # string, the label color. one of ANSI colors.
    label-color:
    # duration, the graph refresh timer.
    refresh-timer: 1s
    # string, one of `overwrite`, `if-not-present`, ``
    # This field allows populating/changing the value of Prefix.Target in the received message.
    # if set to ``, nothing changes 
    # if set to `overwrite`, the target value is overwritten using the template configured under `target-template`
    # if set to `if-not-present`, the target value is populated only if it is empty, still using the `target-template`
    add-target: 
    # string, a GoTemplate that allows for the customization of the target field in Prefix.Target.
    # it applies only if the previous field `add-target` is not empty.
    # if left empty, it defaults to:
    # {{- if index . "subscription-target" -}}
    # {{ index . "subscription-target" }}
    # {{- else -}}
    # {{ index . "source" | host }}
    # {{- end -}}`
    # which will set the target to the value configured under `subscription.$subscription-name.target` if any,
    # otherwise it will set it to the target name stripped of the port number (if present)
    target-template:
    # list of processors to apply on the message before writing
    event-processors: 
    # bool enable debug
    debug: false 
```

### Example

This example shows how to use the `asciigraph` output.

gNMIc config

```shell
cat gnmic_asciiout.yaml
```

```yaml
targets:
  clab-nfd33-spine1-1:
    username: admin
    password: NokiaSrl1!
    skip-verify: true

subscriptions:
  sub1:
    paths:
      - /interface[name=ethernet-1/3]/statistics/out-octets
      - /interface[name=ethernet-1/3]/statistics/in-octets
    stream-mode: sample
    sample-interval: 1s
    encoding: ascii

outputs:
  out1:
    type: asciigraph
    caption: in/out octets per second
    event-processors:
      - rate

processors:
  rate:
    event-starlark:
      script: rate.star
```

Starlark processor

```shell
cat rate.star
```

```python
cache = {}

values_names = [
  '/interface/statistics/out-octets',
  '/interface/statistics/in-octets'
]

N=2

def apply(*events):
  for e in events:
    for value_name in values_names:
      v = e.values.get(value_name)
      # check if v is not None and is a digit to proceed
      if not v:
        continue
      if not v.isdigit():
        continue
      # update cache with the latest value
      val_key = "_".join([e.tags["source"], e.tags["interface_name"], value_name])
      if not cache.get(val_key):
        # initialize the cache entry if empty
        cache.update({val_key: []})
      if len(cache[val_key]) > N:
        # remove the oldest entry if the number of entries reached N
        cache[val_key] = cache[val_key][1:]
      # update cache entry
      cache[val_key].append((int(v), e.timestamp))
      # get the list of values
      val_list = cache[val_key]
      # calculate rate
      e.values[value_name+"_rate"] = rate(val_list)
      e.values.pop(value_name)
    
  return events

def rate(vals):
  previous_value, previous_timestamp = None, None
  for value, timestamp in vals:
    if previous_value != None and previous_timestamp != None:
      time_diff = (timestamp - previous_timestamp) / 1000000000 # 1 000 000 000
      if time_diff > 0:
        value_diff = value - previous_value
        rate = value_diff / time_diff
        return rate

    previous_value = value
    previous_timestamp = timestamp

  return 0
```

<script async id="asciicast-617477" src="https://asciinema.org/a/617477.js"></script>
