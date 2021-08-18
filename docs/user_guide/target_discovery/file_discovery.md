
`gnmic` is able to watch changes happening to a file carrying the gNMI targets configuration.

The file can be located in the local file system or a remote one.

In case of remote file, `ftp`, `sftp`, `http(s)` protocols are supported.
The read timeout of remote files is set to half of the read `interval`
#### Configuration

A file targets loader can be configured in a couple of ways:

- using the `--targets-file` flag:

``` bash
gnmic --targets-file ./targets-config.yaml subscribe
```

``` bash
gnmic --targets-file sftp://user:pass@server.com/path/to/targets-file.yaml subscribe
```

- using the main configuration file:
  
``` yaml
loader:
  type: file
  # path to the file
  path: ./targets-config.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
```

The `--targets-file` flag takes precedence over the `loader` configuration section.

The targets file can be either a `YAML` or a `JSON` file (identified by its extension json, yaml or yml), and follows the same format as the main configuration file `targets` section.
See [here](../../user_guide/targets.md#target-option)

### Examples

#### Local File
``` yaml
loader:
  type: file
  # path to the file
  path: ./targets-config.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
```

#### Remote File

SFTP remote file

``` yaml
loader:
  type: file
  # path to the file
  path: sftp://user:pass@server.com/path/to/targets-file.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
```


FTP remote file

``` yaml
loader:
  type: file
  # path to the file
  path: ftp://user:pass@server.com/path/to/targets-file.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
```

HTTP remote file

``` yaml
loader:
  type: file
  # path to the file
  path: http://user:pass@server.com/path/to/targets-file.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
```

#### Targets file format

=== "YAML"
    ```yaml
    10.10.10.10:
        username: admin
        insecure: true
    10.10.10.11:
        username: admin
    10.10.10.12:
    10.10.10.13:
    10.10.10.14:
    ```
=== "JSON"
    ```json
    {
        "10.10.10.10": {
            "username": "admin",
            "insecure": true
        },
         "10.10.10.11": {
            "username": "admin",
        },
         "10.10.10.12": {},
         "10.10.10.13": {},
         "10.10.10.14": {}
    }
    ```

Just like the targets in the main configuration file, the missing configuration fields get filled with the global flags, 
the ENV variables, the config file main section and then the default values.
