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
