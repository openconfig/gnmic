`gnmic` supports exporting subscription updates to a UDP server

A UDP output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    # required
    type: udp 
    # a UDP server address 
    address: IPAddress:Port
    # maximum sending rate, e.g: 1ns, 10ms
    rate: 10ms 
    # number of messages to buffer in case of sending failure
    buffer-size: 
    # export format. json, protobuf, prototext, protojson, event
    format: json 
    # string, one of `overwrite`, `if-not-present`, ``
    # This field allows populating/changing the value of Prefix.Target in the received message.
    # if set to ``, nothing changes 
    # if set to `overwrite`, the target value is overwritten using the template configured under `target-template`
    # if set to `if-not-present`, the target value is populated only if it is empty, still using the `target-template`
    add-target: 
    # string, a GoTemplate that allow for the customization of the target field in Prefix.Target.
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
    # boolean, valid only if format is `event`.
    # if true, arrays of events are split and marshaled as JSON objects instead of an array of dicts.
    split-events: false
    # boolean, if true the message timestamp is changed to current time
    override-timestamps: false
    # time duration to wait before re-dial in case there is a failure
    retry-interval: 
    # NOT IMPLEMENTED boolean, enables the collection and export (via prometheus) of output specific metrics
    enable-metrics: false 
    # list of processors to apply on the message before writing
    event-processors: 
```

A UDP output can be used to export data to an ELK stack, using [Logstash UDP input](https://www.elastic.co/guide/en/logstash/current/plugins-inputs-udp.html)