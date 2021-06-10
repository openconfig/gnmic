`gnmic` supports dynamic loading of gNMI targets from external systems.

The loaders add/delete gNMI targets without the need to restart `gnmic`.

Three types of target loaders are supported:

- File
- Consul
- Docker

!!! notes
    1.Only one loader is supported at a time.

    2.Target updates are not supported, delete and re-add is the way to update a target configuration.

### File target loader

`gnmic` is able to watch changes happening to a file carrying the gNMI target configuration.

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
  # address of the loader server
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

### Docker target loader

The Docker target loader allows retrieving gNMI targets from [Docker Engine](https://docs.docker.com/engine/) hosts.

The loader discovers containers and the docker network to connect to them, based on a list of docker filters as defined here: [filters](https://docs.docker.com/engine/reference/commandline/ps/#filtering)

Each discovered container is added as a gNMI target.

Example:

```yaml
loader:
  type: docker
  filters:
    # filter 1
    - containers:
        # containers returned by `docker ps -f "label=clab-node-kind=srl"`
        - label: clab-node-kind=srl
      network:
        # networks returned by `docker network ls -f "label=containerlab"`
        label: containerlab
      port: "57400"
      config:
        username: admin
        password: secret1
        skip-verify: true
    # filter 2
    - containers:
        # containers returned by `docker ps -f "label=clab-node-kind=ceos"`
        - label: clab-node-kind=ceos
        # containers returned by `docker ps -f "label=clab-node-kind=vr-sros"`
        - label: clab-node-kind=vr-sros
      network:
        # networks returned by `docker network ls -f "name=mgmt"`
        name: mgmt
      # the value of label=gnmi-port exported by each container`
      port: "label=gnmi-port"
      config:
        username: admin
        password: secret2
        insecure: true
```

The docker loader filter is made of 4 fields:

- **containers**: (Optional)
  
  A list of lists of docker filters used to select containers from the docker engine host.
  If not set, all containers with `status=running` are selected.

- **network**: (Optional)

  A set of docker filters used to select the network to connect to the container.
  If not filter is set, all docker networks are considered.

- **port**: (Optional)

  This field is used to specify the gNMI port for the discovered containers.
  An integer can be specified in which case it will be used as the gNMI port for all discovered containers.
  Alternatively, a string in the format `label=<label_name>` can be set, where `<label_name>` is a docker label containing the gNMI port value.
  If no value is set, the global flag/value `port` is used.

- **config**: (Optional)

  A set of configuration parameters to be applied to all discovered targets by the container filter.
  The same fields as the target configuration section can be specified here.

```yaml
loader:
  # the loader type: docker
  type: docker
  # string, the docker daemon address, l
  # eave empty for the local docker daemon
  address: ""
  # duration, the check interval for discovering 
  # new docker containers, default: 30s
  interval:
  # duration, the docker queries timeout, 
  # defaults to half of `interval` if left unset, 
  # a negative value or a value greater than interval is set
  timeout:
  # bool, print loader debug statements.
  debug:
  # containers, network filters: 
  # see https://docs.docker.com/engine/reference/commandline/ps/#filtering
  # for the possible values.
  filters:
      # containers filters
    - containers:
        # containers returned by `docker ps -f "label=clab-node-kind=srl"`
        - label: clab-node-kind=srl
      # network filters
      network:
        # networks returned by `docker network ls -f "label=containerlab"`
        label: containerlab
      # gNMI port value for the containers discovered by this filter
      port: "57400"
      # target config for containers discovered by this filter.
      config:
        username: admin
        password: secret1
        skip-verify: true
```
