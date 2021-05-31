
The event processors provide an easy way to configure a set of functions in order to transform an event message that will be be written to a specific output.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/processors.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fprocessors.drawio" async></script>


While the `event` format is the de facto format used by `gNMIc` in case the output is `influxdb` or `prometheus`,
it can be used with any other output type.

Transforming the received gNMI message is sometimes needed to accomodate the output system ( converting types, complying with name constraints,...), or simply filtering out values that you are not interested on.

<!-- In certain cases, the gNMI updates received need to be changed or processed to make it easier to ingest by the target output.

Some common use cases:

* Customizing a value or a tag name.
* Converting numbers received as a string to integer or float types. -->

### The event format

The event format is produced by `gNMIc` from the [gNMI Notification messages](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#21-reusable-notification-message-format) received within a gNMI subscribe response update, it contains 5 fields:

* `name`: A `string` field populated by the subscription name, it is used as the measurement name in case of influxdb output or as a part of the metric name in case of prometheus output.
* `timestamp`: An `int64` field containing the timestamp received within the gnmi Update.
* `tags`: A map of string keys and string values. 
The keys and values are extracted from the keys in the [gNMI PathElement](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-path-conventions.md#constructing-paths) keys. `gNMIc` adds the subscription name and the target name/address.
* `values`: A map of string keys and generic values. 
The keys are build from a xpath representation of the gNMI path without the keys, while the values are extracted from the gNMI [Node values](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#223-node-values).
* `deletes`: A `string list` built from the `delete` field of the [gNMI Notification message](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#21-reusable-notification-message-format).


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/event_msg.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fevent_msg.drawio" async></script>

### Defining an event processor

Event processors are defined under the section `processors` in `gNMIc` configuration file.

Each processor is identified by a name, under which we specify the processor type as well as additional fields specific to each type.

!!! note
    Processors names are case insensitive

All processors support a `debug` field that enables extra debug log messages to help troubleshoot the processor transformation.

Below is an example of an `event-delete` processor, which deletes all values with a name containing `multicast` or `broadcast`

```yaml
processors:
  # processor name
  my-processor:
    # processor type
    event-delete:
      value-names:
        - ".*multicast.*"
        - ".*broadcast.*"
```

### Linking an event processor to an output

Once the needed event processors are defined under section `processors`, they can be linked to the desired output(s) in the same file.

Each output can be configured with different event processors allowing flexibility in the way the same data is written to different outputs.

A list of event processors names can be added under an output configuration, the processors will apply in the order they are configured.

In the below example, 3 event processors are configured and linked to `output1` of type `influxdb`.

The first processor converts all value to `integer` if possible.

The second deletes tags with name starting with `subscription-name`. 

Finally the third deletes values with name ending with `out-unicast-packets`.

```yaml
outputs:
  output1:
    type: influxdb
    url: http://localhost:8086
    bucket: telemetry
    token: srl:srl
    batch-size: 1000
    flush-timer: 10s
    event-processors:
      - proc-convert-integer
      - proc-delete-tag-name
      - proc-delete-value-name

processors:
  proc-convert-integer:
    event-convert:
      value-names:
        - ".*"
      type: int

  proc-delete-tag-name:
    event-delete:
      tag-names:
        - "^subscription-name"

  proc-delete-value-name:
    event-delete:
      value-names:
        - ".*out-unicast-packets"
```