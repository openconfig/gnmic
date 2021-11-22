`gnmic` supports exporting subscription updates to multiple local files

A file output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    # required
    type: file 
    # filename to write telemetry data to.
    # will be ignored if `file-type` is set
    filename: /path/to/filename
    # file-type, stdout or stderr.
    # overwrites `filename`
    file-type: # stdout or stderr
    # string, message formatting, json, protojson, prototext, event
    format: 
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
    # string, a GoTemplate that is executed using the received gNMI message as input.
    # the template execution is the last step before the data is written to the file,
    # First the received message is formatted according to the `format` field above, then the `event-processors` are applied if any
    # then finally the msg-template is executed.
    msg-template:
    # boolean, if true the message timestamp is changed to current time
    override-timestamps: 
    # boolean, format the output in indented form with every element on a new line.
    multiline: 
    # string, indent specifies the set of indentation characters to use in a multiline formatted output
    indent: 
    # string, separator is the set of characters to write between messages, defaults to new line
    separator: 
    # integer, specifies the maximum number of allowed concurrent file writes
    concurrency-limit: 1000 
     # boolean, enables the collection and export (via prometheus) of output specific metrics
    enable-metrics: false
     # list of processors to apply on the message before writing
    event-processors:
```

The file output can be used to write to file on the disk, to stdout or to stderr.

For a disk file, a file name is required.

For stdout or stderr, only file-type is required.
