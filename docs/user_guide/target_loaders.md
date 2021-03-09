`gnmic` supports dynamic loading of gNMI targets from external systems.

!!! note
    Only one loader is supported at a time

Two types of target loaders are supported, from file or from a Consul KV store

### File target loader
`gnmic` is able to watch changes happening to a file carrying the gNMI target configuration.

A file targets loader can be configured in a couple of ways:

- Via flag `--targets-file`:

``` bash
gnmic --targets-file ./targets-config.yaml subscribe
```

- Via the main configuration file:
  
``` yaml
loader:
  type: file
  file: ./targets-config.yaml
```

The `--targets-file` flag takes precedence over the `loader` configuration section.

The targets file can be either a `YAML` or a `JSON` file (identified by its extension json, yaml or yml), and follows the same format as the main configuration file `targets` section.
See [here](../user_guide/targets.md#target-option)

Examples:
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


### Consul target loader

The consul target loader is basically `gnmic` watching a [KV](https://www.consul.io/docs/dynamic-app-config/kv) prefix in a `Consul` server.

The prefix is expected to hold each gNMI target configuration as multiple Key/Values.

For example, the below YAML file:
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

is equivalent to the below set of KVs:

| **Key**                                     | **Value** |
| --------------------------------------------|-----------|
| `gnmic/config/targets/10.10.10.10/username` | `admin`   |
| `gnmic/config/targets/10.10.10.10/insecure` | `true`    |
| `gnmic/config/targets/10.10.10.11/username` | `admin`   |
| `gnmic/config/targets/10.10.10.12`          | ""        |
| `gnmic/config/targets/10.10.10.13`          | ""        |
| `gnmic/config/targets/10.10.10.14`          | ""        |


Consul Target loader configuration:

```yaml
loader:
  type: consul
  # address of the locker server
  address: localhost:8500
  # Consul Data center, defaults to dc1
  datacenter: dc1
  # Consul username, to be used as part of HTTP basicAuth
  username:
  # Consul password, to be used as part of HTTP basicAuth
  password:
  # Consul Token, is used to provide a per-request ACL token which overrides the agent's default token
  token:
  # the key prefix to watch for targets configuration, defaults to "gnmic/config/targets"
  key-prefix: gnmic/config/targets
```