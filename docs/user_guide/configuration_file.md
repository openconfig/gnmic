`gnmic` configuration by means of the command line flags is both consistent and reliable. But sometimes its not the best way forward.

With lots of configuration options that `gnmic` supports it might get tedious to pass them all via CLI flags. In cases like that the file-based configuration comes handy.

With a configuration file a user can specify all the command line flags by means of a single file. `gnmic` will read this file and retrieve the configuration options from it.

### What options can be in a file?
Configuration file allows a user to specify everything that can be supplied over the CLI and more.
#### Global flags
All of the [global](#global-flags) flags can be put in a conf file. Consider the following example of a typical configuration file in YAML format:
```yaml
# gNMI target address; CLI flag `--address`
address: "10.0.0.1:57400"
# gNMI target user name; CLI flag `--username`
username: admin
# gNMI target user password; CLI flag `--password`
password: admin
# connection mode; CLI flag `--insecure`
insecure: true
# log file location; CLI flag `--log-file`
log-file: /tmp/gnmic.log
```
With such a file located at a default path the gNMI requests can be made in a very short and concise form:

```bash
# configuration file is read by its default path
gnmi capabilities

# cfg file has all the global options set, so only the local flags are needed
gnmi get --path /configure/system/name
```

#### Local flags
Local flags have the scope of the command where they have been defined. Local flags can be put in a configuration file as well.

To avoid flags names overlap between the different commands a command name should prepend the flag name - `<cmd name>-<flag name>`.

So, for example, we can provide the [`path`](../cmd/get.md#path) flag of a [`get`](../cmd/get.md) command in the file by adding the `get-` prefix to the local flag name:

```yaml
address: "router.lab:57400"
username: admin
password: admin
insecure: true
get-path: /configure/system/name  # `get` command local flag
```

Another example: the [`update-path`](../cmd/set.md#1-in-line-update-implicit-type) flag of a [`set`](../cmd/set.md) will be `set-update-path` in the configuration file.

#### Targets
It is possible to specify multiple targets with different configurations (credentials, timeout,...). This is described in [Multiple targets](targets.md) documentation article.

#### Subscriptions
It is possible to specify multiple subscriptions and associate them with different targets in a flexible way. This configuration option is described in [Multiple subscriptions](subscriptions.md) documentation article.

#### Outputs
The other mode `gnmic` supports (in contrast to CLI) is running as a daemon and exporting the data received from gNMI subscriptions to [multiple outputs](outputs/output_intro.md) like stan/nats, kafka, file, prometheus, influxdb, etc...

#### Inputs
`gnmic` supports reading gNMI data from a set of [inputs](inputs/input_intro.md) and export the data to any of the configured outputs. This is used when building data pipelines with `gnmic`

### Repeated flags
If a flag can appear more than once on the CLI, it can be represented as a list in the file.

For example one can set multiple paths for get/set/subscribe operations. In the following example we define multiple paths for the [`get`](../cmd/get.md) command to operate on:
```yaml
address: "router.lab:57400"
username: admin
password: admin
insecure: true
get-path:
    - /configure/system/name
    - /state/system/version
```

### Options preference
Configuration passed via CLI flags and Env variables take precedence over the file config.

### Environment variables in file
Environment variables can be used in the configuration file and will be expanded at the time the configuration is read.

```yaml
outputs:
  output1:
    type: nats
    address: ${NATS_IP}:4222
```