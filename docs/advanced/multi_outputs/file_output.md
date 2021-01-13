`gnmic` supports exporting subscription updates to multiple local files

A file output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    type: file # required
    filename: /path/to/filename
    file-type: stdout # or stderr
    format: # string, message formatting, json, protojson, prototext, event
    multiline: # string, format the output in indented form with every element on a new line.
    indent: # string, indent specifies the set of indentation characters to use in a multiline formatted output
    separator: # string, separator is the set of charachters to write between messages, defaults to new line
    concurrency-limit: 1000 # integer, specifies the meximum number of allowed concurrent file writes
    enable-metrics: false # NOT IMPLEMENTED boolean, enables the collection and export (via prometheus) of output specific metrics
    event-processors: # list of processors to apply on the mesage before writing

```

The file output can be used to write to file on the disk, to stdout or to stderr.

For a disk file, a file name is required.

For stdout or stderr, only file-type is required.
