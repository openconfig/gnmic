`gnmic` supports exporting subscription updates to multiple local files

A file output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  group1:
    - type: file # required
      file-name: /path/to/filename
      file-type: stdout # or stderr
```

The file output can be used to write to file on the disk, to stdout or to stderr.

For a disk file, a file name is required.

For stdout or stderr, only file-type is required.