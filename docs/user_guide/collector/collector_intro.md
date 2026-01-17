# Collector Mode

## Introduction

The Collector mode (`gnmic collect --config <file_name>`) is ideal for a long-running telemetry collection service.

While the `subscribe` command is designed for interactive use and ad-hoc data collection, the `collect` command is optimized for continuous operation with dynamic configuration capabilities.

## Dynamic Configuration

Unlike gNMIc running with the subscribe command, the collector allows runtime modifications without restarts.

You can add, update or remove **Targets**, **Susbcriptions**, **Outputs**, **Processors** and **Inputs**. All the changes are applied at runtime.

All configuration changes are made via a REST API.

## Clustering

Multiple collector instances can form a cluster just like gNMIc subscribe.

The cluster uses a distributed locker, such as **Consul**, for:

- Leader election
- Target assignment coordination
- Instance membership tracking

## Tunnel Target Support

The collector supports gRPC tunnel, it will accept connections from gNMI tunnel targets.
The tunnel target configutation is done using tunnel-target-matches.

## Comparison with Subscribe Command

| Feature | `subscribe` Command | `collect` Command |
|---------|---------------------|-------------------|
| Configuration | Static (file/flags) | Dynamic (both file and REST API) |
| Target management | Fixed at startup or using loaders | startup file, loaders or REST API |
| Subscription management | Fixed at startup, can be modified using the REST API but requires a target restart to get applied | Add/update/remove at runtime using REST API |
| Output management | Fixed at startup | Add/update/remove at runtime using REST API |
| Tunnel targets | Fixed at startup | dynamic using target tunnel matching rules |

## Getting Started

1. Create a configuration file with at minimum the `api-server` section
2. Start the collector: `gnmic --config collector.yaml collect`
3. Use the REST API or CLI subcommands to manage configuration

See [Collector Configuration](./collector_configuration.md) for detailed configuration options and [Collector REST API](./collector_api.md) for API reference.
