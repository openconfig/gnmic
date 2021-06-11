The Consul target loader is basically `gnmic` watching a [KV](https://www.consul.io/docs/dynamic-app-config/kv) prefix in a `Consul` server.

The prefix is expected to hold each gNMI target configuration as multiple Key/Values.

Putting Key/Values in Consul via the cli is as easy as:

```shell
consul kv put gnmic/config/targets/10.10.10.10/username admin
consul kv put gnmic/config/targets/10.10.10.10/insecure true
consul kv put gnmic/config/targets/10.10.10.11/username admin
consul kv put gnmic/config/targets/10.10.10.12 ""
consul kv put gnmic/config/targets/10.10.10.13 ""
consul kv put gnmic/config/targets/10.10.10.14 ""
```

Verify that keys are present:

```shell
consul kv get -recurse gnmic/config/targets
```

```text
gnmic/config/targets/10.10.10.10/insecure:true
gnmic/config/targets/10.10.10.10/username:admin
gnmic/config/targets/10.10.10.11/username:admin
gnmic/config/targets/10.10.10.12:
gnmic/config/targets/10.10.10.13:
gnmic/config/targets/10.10.10.14:
```

The above command are the equivalent the target YAML file below:

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

### Configuration

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
