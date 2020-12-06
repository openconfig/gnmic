The `event` format is used by `gNMIc` to write into `influxdb` and `prometheus` outputs.

It can also be used with any other output type.

In certain cases, the gNMI updates received need to be changed or processed to make it easier to ingest by the target output.

Some common use cases:

* Customizing a value or a tag name.
* Converting numbers received as a string to integer or float types.


The event processors provide an easy way to configure a set of functions to transform the event message that will be be written to a specific output.

### The event format

The event format is produced by `gNMIc` from the [gNMI Notification messages](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#21-reusable-notification-message-format) received within a gNMI subscribe response update, it contains 5 fields:

* `name`: A `string` field populated by the subscription name, it is used as the measurment name in case of influxdb output or as a part of the metric name in case of prometheus output.
* `timestamp`: An `int64` field containing the timestamp received within the gnmi Update.
* `tags`: A map of string keys and string values. 
The keys and values are extracted from the keys in the [gNMI PathElement](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-path-conventions.md#constructing-paths) keys. `gNMIc` adds the subscription name and the target name/address.
* `values`: A map of string keys and generic values. 
The keys are build from a xpath representation of the gNMI path without the keys, while the values are extracted from the gNMI [Node values](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#223-node-values).
* `deletes`: A `string list` built from the `delete` field of the [gNMI Notification message](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#21-reusable-notification-message-format).


!! DIAGRAM PLACEHOLDER


### Defining an event processor

### Linking an event processor to an output