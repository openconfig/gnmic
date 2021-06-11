
`gnmic` is able to watch changes happening to a file carrying the gNMI targets configuration.

#### Configuration

A file targets loader can be configured in a couple of ways:

- using the `--targets-file` flag:

``` bash
gnmic --targets-file ./targets-config.yaml subscribe
```

- using the main configuration file:
  
``` yaml
loader:
  type: file
  # path to the file
  file: ./targets-config.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 5s
```

The `--targets-file` flag takes precedence over the `loader` configuration section.

The targets file can be either a `YAML` or a `JSON` file (identified by its extension json, yaml or yml), and follows the same format as the main configuration file `targets` section.
See [here](../../user_guide/targets.md#target-option)

### Examples

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
