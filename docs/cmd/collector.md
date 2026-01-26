### Description

The `[collect | collector | coll | c]` command starts gNMIc as a long-running telemetry collector service. Unlike the `subscribe` command which is designed for interactive use, the collector command is optimized for production deployments with dynamic configuration capabilities via REST API.

The collector provides:

- **Dynamic configuration** - Add, modify, or remove targets, subscriptions, outputs, inputs, and processors at runtime via REST API
- **Clustering support** - Multiple collector instances can form a cluster with automatic target distribution and failover
- **Embedded gNMI server** - Expose collected telemetry to downstream gNMI clients
- **Tunnel target support** - Accept connections from gNMI tunnel targets

### Usage

`gnmic [global-flags] collect [local-flags]`

### Local Flags

#### pyroscope-server-address

The `[--pyroscope-server-address]` flag sets the Pyroscope server address for continuous profiling. When set, the collector will send profiling data to the specified Pyroscope server.

#### pyroscope-application-name

The `[--pyroscope-application-name]` flag sets the application name used in Pyroscope. Defaults to `gnmic-collector`.

### Subcommands

The collector command provides subcommands to interact with a running collector instance via its REST API:

| Subcommand | Aliases | Description |
|------------|---------|-------------|
| `targets` | `target`, `tg` | Manage targets |
| `subscriptions` | `subscription`, `sub` | Manage subscriptions |
| `outputs` | `output`, `out` | Manage outputs |
| `inputs` | `input`, `in` | Manage inputs |
| `processors` | `processor`, `proc` | Manage processors |

Each subcommand supports the following operations:

| Operation | Aliases | Description |
|-----------|---------|-------------|
| `list` | `ls` | List all resources |
| `get` | `g`, `show`, `sh` | Get a specific resource |
| `set` | `create`, `cr` | Create or update a resource |
| `delete` | `d`, `del`, `rm` | Delete a resource |

### Configuration

The collector is configured using the standard gNMIc configuration file. The key sections are:

```yaml
# API server configuration (required for collector)
api-server:
  address: :7890
  timeout: 10s
  tls:
    ca-file:
    cert-file:
    key-file:
  enable-metrics: false
  debug: false

# Targets to collect from
targets:
  router1:
    address: 10.0.0.1:57400
    username: admin
    password: admin
    skip-verify: true

# Subscriptions define what data to collect
subscriptions:
  interfaces:
    paths:
      - /interfaces/interface/state/counters
    mode: stream
    stream-mode: sample
    sample-interval: 10s

# Outputs define where to send collected data
outputs:
  prometheus:
    type: prometheus
    listen: :9804
    path: /metrics

# Inputs for receiving data from message queues
inputs:
  nats-input:
    type: nats
    address: nats://localhost:4222
    subject: telemetry.>

# Event processors for data transformation
processors:
  add-hostname:
    event-add-tag:
      tags:
        - tag-name: hostname
          value: ${HOST}

# Clustering configuration (optional)
clustering:
  cluster-name: gnmic-cluster
  instance-name: gnmic-1
  locker:
    type: consul
    address: consul:8500

# gNMI server configuration (optional)
gnmi-server:
  address: :57401
  skip-verify: true
```

### Examples

#### 1. Start a basic collector

```bash
gnmic --config collector.yaml collect
```

#### 2. Start with Pyroscope profiling

```bash
gnmic --config collector.yaml collect \
      --pyroscope-server-address http://pyroscope:4040 \
      --pyroscope-application-name my-collector
```

#### 3. List targets from a running collector

```bash
gnmic --config collector.yaml collect targets list
```

Output:
```
NAME      ADDRESS         USERNAME  STATE    SUBSCRIPTIONS  OUTPUTS  INSECURE  SKIP VERIFY
router1   10.0.0.1:57400  admin     running  2              1        false     true
router2   10.0.0.2:57400  admin     running  2              1        false     true
```

#### 4. Get details of a specific target

```bash
gnmic --config collector.yaml collect targets get --name router1
```

#### 5. Create a new target

```bash
gnmic --config collector.yaml collect targets set --input target.yaml
```

Where `target.yaml` contains:
```yaml
name: router3
address: 10.0.0.3:57400
username: admin
password: admin
skip-verify: true
subscriptions:
  - interfaces
outputs:
  - prometheus
```

#### 6. Delete a target

```bash
gnmic --config collector.yaml collect targets delete --name router3
```

#### 7. List subscriptions

```bash
gnmic --config collector.yaml collect subscriptions list
```

Output:
```
NAME        PREFIX  PATHS                                    ENCODING  MODE           SAMPLE INTERVAL  TARGETS  OUTPUTS
interfaces  -       /interfaces/interface/state/counters     json      stream/sample  10s              2/2      1
```

#### 8. List outputs

```bash
gnmic --config collector.yaml collect outputs list
```

Output:
```
NAME        TYPE        FORMAT  EVENT PROCESSORS
prometheus  prometheus  -       1
```

#### 9. List processors with details

```bash
gnmic --config collector.yaml collect processors list --details
```

### See Also

- [Collector Introduction](../user_guide/collector/collector_intro.md) - Overview and architecture
- [Collector Configuration](../user_guide/collector/collector_configuration.md) - Detailed configuration reference
- [Collector REST API](../user_guide/collector/collector_api.md) - API endpoints reference
