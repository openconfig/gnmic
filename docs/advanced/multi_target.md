Sometimes it is needed to perform an operation on multiple devices; be it getting the same leaf value from a given set of the network elements or setting a certain configuration element to some value.

For cases like that `gnmic` offers support for multiple targets operations which a user can configure both via CLI flags as well as with the [file-based configuration](file_cfg.md).

### CLI configuration
Specifying multiple targets in the CLI is as easy as repeating the [`--address`](../global_flags.md#address) flag.

```
❯ gnmic -a router1.lab.net:57400 \
        -a router2.lab.net:57400 \
        get --path /configure/system/name
```

### File-based configuration
With the file-based configuration a user has two options to specify multiple targets:

* using `address` option
* using `targets` option

#### address option
With `address` option the user must provide a list of addresses. In the YAML format that would look like that:
```yaml
address:
  - "router1.lab.net:57400"
  - "router2.lab.net:57400"
```

The limitation this approach has is that it is impossible to set different credentials for the targets, they will essentially share the credentials specified in a file or via flags.

#### target option
With the `targets` option it is possible to set target specific options (such as credentials, subscriptions, TLS config, outputs), and thus this option is recommended to use:
```yaml
targets:
  router1.lab.net:
    timeout: 2s
    username: r1
    password: gnmi_pass
  router2.lab.net:57000:
    username: r2
    password: gnmi_pass
    tls-key: /path/file1
    tls-cert: /path/file2
```

The target address is defined as the key under the `targets` section of the configuration file. The default port (57400) can be omitted as demonstrated with `router1.lab.net` target address. Have a look at the [file-based targets configuration](https://github.com/karimra/gnmic/blob/master/config.yaml) example to get a glimpse of what it is capable of.

The target inherits the globally defined options if the matching options are not set on a target level. For example, if a target doesn't have a username defined, it will use the username value set on a global level.

Target supported options:
```yaml
targets:
  target1:
    name:
    address:
    username:
    password:
    timeout:
    insecure:
    tls-ca:
    tls-cert:
    tls-key:
    tls-max-version:
    tls-min-version:
    tls-version:
    skip-verify:
    subscriptions:
    outputs:
    buffer-size:
    retry:
```

### Example
Whatever configuration option you choose, the multi-targeted operations will uniformly work across the commands that support them.

Consider the `get` command acting on two routers getting their names:
```
❯ gnmic -a router1.lab.net:57400 \
        -a router2.lab.net:57400 \
        get --path /configure/system/name

[router1.lab.net:57400] {
[router1.lab.net:57400]   "source": "router1.lab.net:57400",
[router1.lab.net:57400]   "timestamp": 1593009759618786781,
[router1.lab.net:57400]   "time": "2020-06-24T16:42:39.618786781+02:00",
[router1.lab.net:57400]   "updates": [
[router1.lab.net:57400]     {
[router1.lab.net:57400]       "Path": "configure/system/name",
[router1.lab.net:57400]       "values": {
[router1.lab.net:57400]         "configure/system/name": "gnmic_r1"
[router1.lab.net:57400]       }
[router1.lab.net:57400]     }
[router1.lab.net:57400]   ]
[router1.lab.net:57400] }

[router2.lab.net:57400] {
[router2.lab.net:57400]   "source": "router2.lab.net:57400",
[router2.lab.net:57400]   "timestamp": 1593009759748265232,
[router2.lab.net:57400]   "time": "2020-06-24T16:42:39.748265232+02:00",
[router2.lab.net:57400]   "updates": [
[router2.lab.net:57400]     {
[router2.lab.net:57400]       "Path": "configure/system/name",
[router2.lab.net:57400]       "values": {
[router2.lab.net:57400]         "configure/system/name": "gnmic_r2"
[router2.lab.net:57400]       }
[router2.lab.net:57400]     }
[router2.lab.net:57400]   ]
[router2.lab.net:57400] }
```

Notice how in the output the different gNMI targets are prefixed with the target address to make the output easy to read. If those prefixes are not needed, you can make them disappear with [`--no-prefix`](../global_flags.md#no-prefix) global flag.