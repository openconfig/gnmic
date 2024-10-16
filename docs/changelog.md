## Changelog

### v0.38.0 - July 8th 2024

- Kafka Output

    - Add configurable Kafka version

- gNMI extensions

    - Implement [Commit confirmed gNMI extension](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-commit-confirmed.md)
    - Implement [Depth gNMI extension](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-depth.md)

### v0.37.0 - May 13th 2024

- gNMI connection TCP Keepalive

    - It is now possible to configure the TCP keepalive probes time interval.

- gRPC Keepalive

    - The gRPC connection keepalive parameters are now configurable.
    It follows the gRPC spec: https://github.com/grpc/proposal/blob/master/A8-client-side-keepalive.md

- Proxy Command:

    - gNMIc now supports a `proxy` command.
    When issued, gNMIc runs as a gNMI proxy. See details [here](cmd/proxy.md)

- Processor Command:

    - gNMIc now supports a `processor` command.
    It can be used to run a set of processor offline against an input of event messages and print the result.
    See details [here](cmd/processor.md)

- Kafka Output:

    - The Kafka output now supports a configurable flush-interval parameters.

- InfluxDB Output:

    - The InfluxDB output now supports writing gNMI deletes to InfluxDB using a custom tag name.

- Prometheus Output:

    - The Prometheus output will now automatically convert boolean values (true: 1 and false: 0).


### v0.36.0 - February 13th 2024

- Event Message

    - gNMI updates with deleted paths are now converted into separate event messages where the keys are extracted from the path and set as event tags.

- gNMI TLS cipher suites

    - It is now possible to select the list of a cipher suites that gNMIc advertises to a gNMI server during a TLS handshake. The full list of supported ciphers can be found [here](user_guide/targets/targets.md#controlling-the-advertised-cipher-suites)

- Set Request

    - The Set command now features a new flag, `--proto-file`, which allows the specification of one or more files. These files should contain gNMI Set requests in `prototext` format, which will be sent to the specified targets.

### v0.35.0 - January 20th 2024

- Processors

    - Added a plugin process type that allows users to write their own custom processors: [examples](https://github.com/openconfig/gnmic/tree/main/examples/plugins)

- gRPC metadata

    - A new flag `--metadata | -H` is introduced. It allows users to add custom gRPC metadata headers to any request.


- Outputs:

    - Kafka output:
        - Added support for custom topics per target/subscription.

        - Added support for both Async and Sync Kafka producers.

- Commands:

    - Listen command:
        When using the `listen` command outputs internal metrics are properly initialized and exposed to prometheus for scraping.

### v0.34.0 - November 11th 2023

- Prometheus Write Output

    - The number of `prometheus_write` writers can now be configured.

- Subscription Encoding

    - A subscription encoding can now be set per target. Before, it was either a global attribute or set per subscription.
        With this change, it can be set globally, per target or per subscription.

- Processors:

    - New `event-combine` processor: A convenience processor that allows combining other processors into a single one.

    - New `event-rate-limit` processor: A processor that rate-limits each event with matching tags to the configured amount per-seconds.

- Outputs:

    - New `asciigraph` output: https://asciinema.org/a/617477

- Clustering:

    - New `redis` locker: For leader election, service discovery and target distribution gNMIc supports both `Consul` and `Kubernetes`. It is now possible to use `redis` for the same purpose.

### v0.33.0 - October 8th 2023

- Rest API

    - Added a kubernetes friendly `api/v1/healthz` endpoint.

- Set Command

    - Added support for gNMI set [`union_replace`](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-union_replace.md) operation.

- Outputs

    - Allow the number of workers used by the `prometheus` and `prometheus_write` outputs to be configurable to improve performance.

- Go version

    - Upgrade to Golang v1.21.1.

### v0.32.0 - August 31st 2023

- TLS

    - It is now possible to override the serverName used by gNMIc when verifying the server name present in the
      certificate sent by the gNMI server. [PR](https://github.com/openconfig/gnmic/pull/173)

- Subscription
 
    - Added support for mixing on-change and sample stream subscription in the same gRPC stream. [PR](https://github.com/openconfig/gnmic/pull/197)
    - Added support for attaching specific outputs to a subscription. [PR](https://github.com/openconfig/gnmic/pull/209)

- REST API

    - Added a health chek endpoint to be used by kubernetes. [PR](https://github.com/openconfig/gnmic/pull/202)

- Kafka Output

    - Added support for Kafka compression. [PR](https://github.com/openconfig/gnmic/pull/203)

- Generate Path
 
    - Added `enum-values` to the `JSON` output of `generate path` command. [PR](https://github.com/openconfig/gnmic/pull/215)

### v0.31.0 - May 17th 2023

- Prometheus output

    - When using the Consul auto discovery feature of the Proemtheus output,
      it is now possible to configure different service and listen addresses.
      This is useful when gNMIc is running as a container of behind a NAT device

- Set Request file

    - The CLI origin is now allowed in the `path` field of `updates`, `replaces` and `deletes` in a set request file.
      If the `path` field has the `cli:/` origin, the `value` field is expected to be a string and will be set in an `ascii` TypedValue.

### v0.30.0 - April 18th 2023

- Set Command

    - The [set command](cmd/set.md) now supports the flags `--replace-cli`, `--replace-cli-file`, `--update-cli` and `--update-cli-file`, these flags can be used to send gNMI set requests with the CLI origin.

- Logging:
  
    - Reduce log verbosity of File and HTTP target discovery mechanisms.

- Processors:

    - The [Drop](user_guide/event_processors/event_drop.md) event processor completely removes the message to be dropped instead of replacing it with an empty message.

- Inputs:

    - [Kafka input](user_guide/inputs/kafka_input.md) now supports TLS connections.

- Outputs:

    - [Kafka output](user_guide/outputs/kafka_output.md) now has a configuration attribute called `insert-key`, if true, the messages written will include a key built from the gNMI message source and subscription name.

    - [TCP output](user_guide/outputs/tcp_output.md) now has a configuration attribute called `delimiter`, it allows to set user defined string to be sent between each message. This allows the receiving end to properly split JSON objects. It it particularly useful with Logstash when writing gNMI events to an ELK stack.

- TLS:

    - When using `gNMIc`'s components that expose a TLS server (gNMI server, Tunnel server, Rest API and Prometheus output) it's possible to fine tune the how the server requests and validates a client certificate.

        This is done using the configuration attribute `client-auth` under each server's TLS section, it takes 4 different values:

        - request:
            The server requests a certificate from the client but does not require the client to send a certificate.
            If the client sends a certificate, it is not required to be valid.

        - require:
            The server requires the client to send a certificate and does not fail if the client certificate is not valid.

        - verify-if-given:
            The server requests a certificate, does not fail if no certificate is sent. If a certificate is sent it is required to be valid.

        - require-verify:
            The server requires the client to send a valid certificate.

- Diff Command:

    - The [diff command](cmd/diff/diff.md) has 2 new sub commands:

        - [`setrequest`](cmd/diff/diff_setrequest.md): compares the intent between two `SetRequest` messages encoded in textproto format.

        - [`set-to-notifs`](cmd/diff/diff_set_to_notifs.md): verifies whether a set of
            notifications from a `GetResponse` or a stream of `SubscribeResponse` messages
            comply with a `SetRequest` messages in textproto format. The envisioned use case
            is to check whether a stored snapshot of device state matches that of the
            intended state as specified by a `SetRequest`.

- Outputs:

    - When using the `event` format with certain outputs (`file`, `nats`, `jetstream`, `kafka`, `tcp` or `udp`) it's possible to send event message individually as opposed to sending them in an array.
        This is done using the attribute `split-events: true` under each of the outputs configuration sections.

    - [Prometheus output](user_guide/outputs/prometheus_output.md) now supports a custom service address field under `service-registration`, it specifies the address to be registered in Consul for discovery.
        It can be a hostname, an IP address or a IP/Host:Port socket address. It it does not contain a port number, the port number from the `listen` field is used.

- Set Request file

    - The Set request file can be used with Origin `cli`, gNMIc will properly format the commands as string, not as JSON value.

### v0.29.0 - February 20th 2023

- Generate Path

    - The `generate path` command with the flag `--json` shows the features the path depends on.
      The list of features is built recursively from the YANG attribute `if-feature`.

- Processors:

    - New processor [`event-starlark`](user_guide/event_processors/event_starlark.md) allows to run a [starlak](https://github.com/google/starlark-go/blob/master/doc/spec.md) script on the received messages.

- Loaders

    - The [HTTP loader](user_guide/targets/target_discovery/http_discovery.md) now supports different authentication schemas as well as setting a template from a local file.

### v0.28.0 - December 7th 2022

- Targets

    - Targets static tags are now properly propagated to outputs when a cache is used.

- Listen Command:

    - The `system-name` HTTP2 header is now used as a tag in exported metrics.

- Outputs:

    - The timestamp precision under `gNMIc`'s InfluxDB output is now configurable.

    - Added a new `snmp` output type, it allows to dynamically convert gNMI updates into SNMP traps.

### v0.27.0 - October 8th 2022

- Targets

    - Add supports for socks5 proxies per target.

- Logging

    - Support for log rotation via the flags `--log-max-size`, `log-max-backups` and `--log-compress`

### v0.26.0 - June 28th 2022

- Outputs

    - Add [Prometheus Remote Write output](user_guide/outputs/prometheus_write_output.md), this output type can be used to push metrics to various systems like [Mimir](https://grafana.com/oss/mimir/), [CortexMetrics](https://cortexmetrics.io/), [VictoriaMetrics](https://victoriametrics.com/), [Thanos](https://thanos.io/)...
    - Add [NATS Jetstream output](user_guide/outputs/jetstream_output.md), it allows to write metrics to NATS jetstream which supports persistency and filtering.

- [gNMI historical subscriptions](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-history.md#1-purpose)

    `gNMIc` now support historical subscription using the [gNMI history extension](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-history.md#2-definition)

### v0.25.1 - June 13th 2022

- Upgrade Go version to go1.18.1.

- Fix running `gnmic subscribe` with only Inputs and Outputs configured (no subscriptions or targets).

### v0.25.0 - June 11th 2022

- Processors

    - [Strings replace processor](user_guide/event_processors/event_strings.md) supports replaces using regular expressions.

    - Processors  are now supported when collecting telemetry using [listen command](cmd/listen.md) (Nokia SROS specific)

- New Processors

    - [Data convert](user_guide/event_processors/event_data_convert.md)

    - [Duration convert](user_guide/event_processors/event_duration_convert.md)
    
    - [Value tag](user_guide/event_processors/event_value_tag.md)

- [Clustering](user_guide/HA.md)

    - `gNMIc` supports kubernetes based clustering,
    i.e you can build `gNMIc` clusters on kubernetes without the need for Consul cluster.

- [Yang path generation](cmd/generate.md)

    - The command `gnmic generate path` supports generating paths for YANG containers.
    In earlier versions, the paths generation was done for YANG leaves only.

- Internal gNMIc Prometheus metrics

    `gNMIc` exposes additional internal metrics available to be scraped using Prometheus.

- Static tags from target configuration

    - It is now possible to set static tags on events by configuring them under each target.

- Influxdb cache

    The [InfluxDB output](user_guide/outputs/influxdb_output.md) now supports gNMI based caching, allowing to apply processors on multiple event messages at once and batching the written points to InfluxDB.

### v0.24.0 - March 13th 2022

- [gRPC Tunnel Support](user_guide/tunnel_server.md)

    Add support for gNMI RPC using a gRPC tunnel, gNMIc runs as a collector with an embedded tunnel server.

### v0.23.0 - February 24th 2022

- Docker image:

    - The published `gnmic` docker image is now based on `alpine` instead of an empty container.
    - A `from scratch` image is published and can be obtained using the command:
     ```bash
     docker pull ghcr.io/karimra/gnmic:latest-scratch
     docker pull ghcr.io/karimra/gnmic:v0.23.0-scratch
     ```

- [gNMIc Golang API](user_guide/golang_package/intro.md):

    - Add gNMI responses constructors
    - Add gRPC tunnel proto messages constructors

- [Target Discovery](user_guide/targets/target_discovery/discovery_intro.md):

    - Add the option to transform the loaded targets format using a Go text template for file and HTTP loaders
    - Poll based target loaders (file, HTTP and docker) now support a startup delay timer

### v0.22.1 - February 2nd 2022

- Fix a Prometheus output issue when using gNMI cache that causes events to be missing from the metrics.

### v0.22.0 - February 1st 2022

- [gNMIc Golang API](user_guide/golang_package/intro.md):

    Added the `github.com/karimra/gnmic/api` golang package.
    It can be imported by other Golang programs to ease the creation of gNMI targets and gNMI Requests.

### v0.21.0 - January 23rd 2022

- [Generate Cmd](cmd/generate/generate_path.md):

    Add YANG module namespace to generated paths.

- Outputs:

    Outputs [File](user_guide/outputs/file_output.md), [NATS](user_guide/outputs/nats_output.md) and [Kafka](user_guide/outputs/kafka_output.md) now support a `msg-template` field to customize the written messages using Go templates.

- API:

    Add [Cluster API](user_guide/api/cluster.md) endpoints.

- Actions:

    Add [Template](user_guide/actions/actions.md#template-action) action.

    Add Subscribe ONCE RPC to [gNMI](user_guide/actions/actions.md#gnmi-action) action.

    Allow [gNMI](user_guide/actions/actions.md#gnmi-action) action on multiple targets.

    Add [Script](user_guide/actions/actions.md#script-action) action.

- [Get Cmd](cmd/get.md):

    Implement Format `event` for GetResponse messages.

    Add the ability to execute processors with Get command flag [`--processor`](cmd/get.md#processor) on GetResponse messages.

- [Target Discovery](user_guide/targets/target_discovery/discovery_intro.md):

    Add the ability to run [actions](user_guide/actions/actions.md) on target discovery or deletion.

- [Set Cmd](cmd/set.md):

    Add [`--dry-run`](cmd/set.md#dry-run) flag which runs the set request templates and prints their output without sending the SetRequest to the targets.

- TLS:

    Add pre-master key logging for TLS connections using the flag [`--log-tls-secret`](global_flags.md#log-tls-secret). The key can be used to decrypt encrypted gNMI messages using wireshark.

- Target:

    Add `target.Stop()` method to gracefully close the target underlying gRPC connection.

### v0.20.0 - October 19th 2021

- Add [gomplate](https://docs.gomplate.ca) template functions to all templates rendered by `gnmic`.

- [Path generation](cmd/generate/generate_path.md):

    `gnmic generate path` supports generating paths with type and description in JSON format.

- [Set RPC template](cmd/set.md#templated-set-request-file):

    Set RPC supports multiple template files in a single command.

- [Clustering](user_guide/HA.md):

    `gnmic` clusters can be formed using secure (HTTPS) API endpoints.

- [Configuration payload generation](cmd/generate.md):

    Configuration keys can now be formatted as `camelCase` or `snake_case` strings

### v0.19.1 - October 7th 2021

- Path search
  
  Do not enter search mode if not paths are found.

- [Prometheus Output](user_guide/outputs/prometheus_output.md)

  Change the default service name when registering with a Consul server

### v0.19.0 - September 16th 2021

- Event Processors

    [Event Convert](user_guide/event_processors/event_convert.md) now converts binary float notation to float

- Target Loaders:

    - [HTTP Loader](user_guide/targets/target_discovery/http_discovery.md)

      gNMIc can now dynamically discover targets from a remote HTTP server.

      HTTP Loader is now properly instrumented using Prometheus metrics.

    - [File Loader](user_guide/targets/target_discovery/file_discovery.md)

      Supports remote files (ftp, sftp, http(s)) in addition to local file system files.

      File loader is now properly instrumented using Prometheus metrics.

    - [Consul Loader](user_guide/targets/target_discovery/consul_discovery.md)

      Consul Loader is now properly instrumented using Prometheus metrics.

    - [Docker Loader](user_guide/targets/target_discovery/docker_discovery.md)

      Docker Loader is now properly instrumented using Prometheus metrics.

- gRPC

     gNMIc now adds its version as part of the user-agent HTTP header.

### v0.18.0 - August 17th 2021

- [gNMI Server](user_guide/gnmi_server.md):

    Add support for a global gNMI server.
    It supports all types of subscriptions, ran against a local cache build out the configured subscriptions.
    It support Get and Set RPCs as well, those are run against the configured targets.

    The gNMI server supports Consul based service registration.

- Outputs:

    Add support for [gNMI server](user_guide/outputs/gnmi_output.md) output type

- [Target configuration](user_guide/targets/targets.md):

    Support multiple IP addresses per target, all addresses are tried simultaneously.
    The first successful gRPC connection is used.

- [Prometheus Output](user_guide/outputs/prometheus_output.md):

    Add the option of generating Prometheus metrics on-scrape, instead of on-reception.
    The gNMI notifications are stored in a local cache and used to generate metrics when a Prometheus server sends a scrape request.

- Event Processors:

    Add [`group-by`](user_guide/event_processors/event_group_by.md) processor, it groups events together based on a given criteria.
    The events can belong to different gNMI notifications or even to different subscriptions.

- Event Processor Convert:

    Add support for boolean conversion

- [Deployment Examples](deployments/deployments_intro.md):

    Add [containerlab](https://containerlab.srlinux.dev) based deployment examples.
    These deployment come with a router fabric built using Nokia's [SRL](https://learn.srlinux.dev)

- [API server](user_guide/api/api_intro.md):

    Add Secure API server configuration options

- Target Loaders:

    [Consul loader](user_guide/targets/target_discovery/consul_discovery.md#services-watch) update: Add support for gNMI target discovery from Consul services.

- Get Request:

    Add printing of Target as part of Path Prefix

- Set Request:

    Add printing of Target as part of Path Prefix

### v0.17.0 - July 14th 2021

- Event Trigger:

    Enhance `event-trigger` to run multiple actions sequentially when an event occurs.

    The output of an action can be used in the following ones.

- Kafka output:

    Add `SASL_SSL` and `SSL` security protocols to kafka output.

- gRPC authentication:

    Add support for token based gRPC authentication.

### v0.16.2 - July 13th 2021

- Fix nil pointer dereference in case a subscription has `suppress-redundant` but no `heartbeat-interval`.

### v0.16.1 - July 12th 2021

- Bump github.com/openconfig/goyang version to v0.2.7
  
### v0.16.0 - June 14th 2021

- Target Discovery:

    Add Docker Engine target loader, `gnmic` can dynamically discover gNMI targets running as docker containers.

- Event Trigger: gNMI action

    Enhance `gNMI action` to take external variables as input, in addition to the received gNMI update.

### v0.15.0 - June 7th 2021

- Subscription:

   Add field `set-target` under subscription config, a boolean that enables setting the target name as a gNMI prefix target.

- Outputs:

   Add `add-target` and `target-template` fields under all outputs,
   Enables adding the target value as a tag/label based on the subscription and target metadata

### v0.14.3 - June 6th 2021

- Set command:

    Fix `ascii` values encoding if used with `--request-file` flag.

### v0.14.2 - June 3rd 2021

- Fix `event-convert` processor when the conversion is between integer types.
- Add an implicit conversion of uint to int if the influxdb output version is 1.8.x.
  This is a workaround for the limited support of influx APIv2 by influxDB1.8

### v0.14.1 - May 31st 2021

- Fix OverrideTS processor
- Add `override-timestamps` option under outputs, to override the message timestamps regardless of the message output format

### v0.14.0 - May 28th 2021

- New Output format `flat`
    - This format prints the Get and Subscribe RPCs as a list of `xpath: value`, where the `xpath` points to a leaf value.

- New `gnmic diff` command:
    - This command prints the difference in responses between a reference target `--ref` and one or more targets to be compared to the reference `--compare`.
    - The output is printed as `flat` format results.
  
### v0.13.0 - May 10th 2021

- New `gnmic generate` Command:
    - Given a set of yang models and an xpath, `gnmic generate` generates a JSON/YAML representation of the YANG object the given path points to.
    - Given a set of yang models and an set of xpaths (with `--update` or `--replace`), `gnmic generate set-request` generates a set request file that can be filled with the desired values and used with `gnmic set --request-file`
    - The sub-command `gnmic generate path` is an alias to `gnmic path`

- Path Command:
    - add flag `--desc` which, if present, prints the YANG leaf description together with the generated paths.
    - add flag `--config-only` which, if present, only generates paths pointing to YANG leaves representing config data.
    - add flag `--state-only` which, if present, only generates paths pointing to a YANG leaf representing state data.

### v0.12.2 - April 24th 2021

- Fix a bug that cause gNMIc to crash if certain processors are used.

### v0.12.1 - April 21st 2021

- Fix parsing of stringArray flags containing a space.

### v0.12.0 - April 20th 2021

- Outputs:
    - InfluxDB and Prometheus outputs: Convert gNMI Decimal64 values to Float64.
- Set Command:
    - Add the ability to run a Set command using a single file, including `replaces`, `updates` and `deletes`.
    - The request file `--request-file` is either a static file or a Golang Text Template rendered separately for each target.
    - The template input is read from a file referenced by the flag `--request-vars`.
  
### v0.11.0 - April 15th 2021

- Processors:
    - Add `event-allow` processor, basically an allow ACL based on `jq` condition or regular expressions.
    - Add `event-extract-tags` processor, it adds tags based on regex named groups from tag names, tag values, value names, or values.
    - Add `gnmi-action` to `event-trigger` processor, the action runs a gNMI Set or Get if the trigger condition is met.
- Set Command:
    - Improve usability by supporting reading values (--update-file and --replace-file) from standard input.

### v0.10.0 - April 8th 2021

- New command:
    - `getset` command: This command conditionally executes both a `Get` and a `Set` RPC, the `GetResponse` is used to evaluate a condition which if met triggers the execution of the `Set` RPC.
- Processors:
    - Some processors' apply condition can be expressed using `jq` instead of regular expressions. 

### v0.9.1 - March 23rd 2021

- Processors:
    - Add `event-trigger` processor: This processor is used to trigger a predefined action if a condition is met.
    - New processor `event-jq` which applies a transformation on the messages expressed as a jq expression.
    
- Shell autocompletion:
    - Shell (bash, zsh and fish) autocompletion scripts can be generated using `gnmic completion [bash|zsh|fish]`.
- gRPC gzip compression:
    - `gnmic` supports gzip compression on gRPC connections.
  
### v0.9.0 - March 11th 2021

- Clustered Prometheus output:
    - When deployed as a cluster, it is possible to register only one of the prometheus outputs in Consul. This is handy in the case of a cluster with data replication.
- Proto file loading at runtime (Nokia SROS):
    - `gnmic` supports loading SROS proto files at runtime to decode gNMI updates with `proto` encoding
- Kafka Output:
    - Kafka SASL support: PLAIN, SCRAM SHA256/SHA512 OAuth mechanisms are supported.
- Configuration:
    - `gnmic` supports configuration using environment variables.
- Processors:
    - add `event-merge` processor.
- Target Loaders:
    - `gnmic` supports target loaders at runtime, new targets can be added to the configuration from a file that `gnmic` watches or from `Consul`
  
### v0.8.0 - March 2nd 2021

- Inputs:
    - Processors can now be applied by the input plugins.
- Prometheus output:
    - The Prometheus output can now register as a service in Consul, a Prometheus client can discover the output using consul service discovery.
- Clustering:
    - `gnmic` can now run as a cluster, this requires a running Consul instance that will be used by the `gnmic` instance for leader election and target load sharing.
- Configuration file:
    - The default configuration file placement now follows [XDG](https://wiki.archlinux.org/index.php/XDG_Base_Directory) recommendations
- CLI exit status:
    - Failure of most commands is properly reflected in the cli exit status.
- Configuration:
    - Configuration fields that are OS paths are expanded by `gnmic`
- Deployment examples:
    - A set of deployment examples is added to the repo and the docs.
  
### v0.7.0 - January 28th 2021

- Prometheus output metrics customization:
    - `metric-prefix` and `append-subscription-name` can be used to change the default metric prefix and append the subscription name to the metric name.
    - `export-timestamps`: enables/disables the export of timestamps together with the metric.
    - `strings-as-labels`: enables/disables automatically adding paths with a value of type string as a metric label.

- NATS output:
    - allow multiple NATS workers under NATS output via field `num-workers`.
    - add NATS prometheus internal metrics.

- STAN output:
    - allow multiple STAN workers under STAN output via field `num-workers`.
    - add NATS prometheus internal metrics.

- File output:
    - add File prometheus metrics.
  
- Inputs:
    - support ingesting gNMI data from NATS, STAN or a Kafka message bus.


### v0.6.0 - December 14th 2020

- Processors:
    - Added processors to `gnmic`, a set of basic processors can be used to manipulate gNMI data flowing through `gnmic`. These processors are applied by the output plugins

- Upgrade command: `gnmic` can be upgraded using `gnmic version upgrade` command.

### v0.5.2 - December 1st 2020
- Outputs:
    - Improve outputs logging
    - Add Prometheus metrics to Kafka output

### v0.5.1 - November 28th 2020
- Prompt Mode:
    - Fix subscribe RPC behavior
- QoS:
    - Do not populate QoS field if not set via config file or flag.
Outputs:
    - add configurable number of workers to some outputs.

### v0.5.0 - November 25th 2020
- Prompt Mode:
    - Add prompt sub commands.
- XPATH parsing:
    - Add custom xpath parsingto gnmi.Path to allow for paths including column `:`.
- TLS:
    - Allow configurable TLS versions per target, the minimum, the maximum and the preferred TLS versions ca be configured.
  
### v0.4.3 - November 10th 2020
- Missing path:
    - Initialize the path field if not present in SubscribeResponse
  
### v0.4.2 - November 5th 2020
- YANG:
    - Prompt command flags `--file` and `--dir` support globs.
- Subscribe:
    - added flags `--output` that allows to choose a single output for `subscribe` updates
- Prompt:
    - Max suggestions is automatically adjusted based on the terminal height.
    - Add suggestions for address and subscriptions.

### v0.4.1 - October 22nd 2020
- Prompt:
    - Add suggestions of xpath with origin, `--suggest-with-origin`.
### v0.4.0 - October 21st 2020
- New Command:
    - Add new command `prompt`
- Prompt:
    - Add ctrl+z key bind to delete a single path element.
    - Add YANG info to xpath suggestions.
    - Add GoLeft, GoRight key binds.
    - Sort xpaths and prefixes suggestions.
    - xpaths suggestions are properly generated if a prefix is present.
    - flag `--suggest-all-flags` allows adding global flags suggestion in prompt mode.
  
- Prometheus output:
    - Add support for Prometheus output plugin.
    
### v0.3.0 - October 1st 2020
- InfluxDB output:
    - Add support for influxDB output plugin.

### v0.2.3 - September 18th 2020
- Retry
    - Add basic RPC retry mechanism.
- ONCE mode subscription:
    - Handle targets that send an EOF error instead of a SyncResponse to signify the end of ONCE subscriptions.
- Docker image:
    - Docker images added to ghcr.io as well as docker hub.
  
### v0.2.2 - September 3rd 2020
- CLI:
    - Properly handle paths that include quotes.
- Unix Socket:
    - Allow send/rcv of gNMI data to/from a unix socket.
- Outputs:
    - Add TCP output plugin.

### v0.2.1 - August 11th 2020
- Releases:
    - Add .deb. and .rpm packages to releases.
 - Outputs:
    - Add UDP output plugin. 

### v0.2.0 - August 7th 2020
- Releases:
    - Add ARM releases.
    - Push docker image to docker hub.
  
### v0.1.1 - July 23rd 2020
- Set Cmd:
    - Support `json_ietf` encoding when the value is specified from a file.
  
### v0.1.0 - July 16th 2020
- Outputs:
    - Allow NATS/STAN output subject customization.

### v0.0.7 - July 16th 2020
- gNMI Target:
    - Add support for gNMI Target field.
- gNMI Origin:
    - Add support for gNMI Origin field.  
- Prometheus internal metrics:
    - Add support for `gnmic` internal metrics via a Prometheus server.
- Outputs:
    - Add support for multiple output plugins (file, NATS, STAN, Kafka)
- Targets:
    - Support target specific configuration.
- Poll Subscription:
    - Allow selecting polled targets and subscription using a CLI select menu.
- gNMI Models:
    - Support multiple Models in Get and Subscribe RPCs.

### v0.0.6 - June 2nd 2020
- Nokia Dialout:
    - Add Support for Nokia Dialout telemetry.
- Printing:
    - Convert timestamps to Time.

### v0.0.5 - May 18th 2020
- Formatting:
    - Add `textproto` format.

### v0.0.4 - May 11th 2020
- Logging:
    - Support logging to file instead of Stderr.
- Set Command:
    - support Set values from YAML file.

### v0.0.3 - April 23rd 2020
- Proxy:
    - Allow usage of ENV proxy values for gRPC connections.
- Installation:
    - Add installation script.

### v0.0.2 - April 13th 2020
- Terminal printing clean up.
- Path Command: Add search option.

### v0.0.1 - March 24th 2020
- Capabilities RPC Command.
- Get RPC Command.
- Subscribe RPC Command.
- Set RPC Command.
- TLS support.
- Version Command.
- Path Commnd.

### initial Commit - February 20th 2020
