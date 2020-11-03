`gnmic` configuration by means of the command line flags is both consistent and reliable. But sometimes its not the best way forward.

With lots of configuration options that `gnmic` supports it might get tedious to pass them all via CLI flags. In cases like that the file-based configuration comes handy.

With a configuration file a user can specify all the command line flags by means of a single file. `gnmic` will read this file and retrieve the configuration options from it.

### File type, name and location
Configuration file that `gnmic` reads must be in one of the following formats: JSON, YAML, TOML, HCL or Properties.  
By default, `gnmic` will check if the file referenced by `$HOME/gnmic.[yml, toml, json]` path exists. The default path can be overridden with [`--config`](../global_flags.md#config) flag.

```bash
# config file default path is ~/gnmic.[yml, toml, json]
gnmic capabilities

# read `cfg.yml` file located in the current directory
gnmic --config ./cfg.yml capabilities
```

If the file referenced by `--config` flag is not present, the default path won't be tried.

Example of the `gnmic` config files are provided in the following formats: [YAML](https://github.com/karimra/gnmic/blob/master/config.yaml), [JSON](https://github.com/karimra/gnmic/blob/master/config.json), [TOML](https://github.com/karimra/gnmic/blob/master/config.toml).

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

#### Subscriptions
It is possible to specify multiple subscriptions and associate them with multiple targets in a flexible way. This advanced technique is described in [Multiple subscriptions](subscriptions.md) documentation article.

### Outputs
The other mode `gnmic` supports (in contrast to CLI) is running as a daemon and exporting the data received with gNMI subscriptions to multiple outputs like stan/nats, kafka, file, etc.

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
Configuration passed via CLI flags takes precedence over the file config.
