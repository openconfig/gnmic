`gnmic` supports exporting subscription updates as [OpenTelemetry](https://opentelemetry.io/) metrics using the [OTLP](https://opentelemetry.io/docs/specs/otlp/) protocol.

This output can be used to push metrics to any OTLP-compatible backend such as [Grafana Alloy](https://grafana.com/docs/alloy/latest/), [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/), [Datadog](https://www.datadoghq.com/), [Dynatrace](https://www.dynatrace.com/), or any system that accepts OTLP metrics over gRPC.

## Configuration

An OTLP output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    # required
    type: otlp
    # required, address of the OTLP collector
    endpoint: localhost:4317
    # string, transport protocol. Only "grpc" is supported.
    # defaults to "grpc"
    protocol: grpc
    # duration, defaults to 10s.
    # RPC timeout for each export request.
    timeout: 10s
    # tls config
    tls:
      # string, path to the CA certificate file,
      # this will be used to verify the server certificate when `skip-verify` is false
      ca-file:
      # string, client certificate file.
      cert-file:
      # string, client key file.
      key-file:
      # boolean, if true, the client will not verify the server
      # certificate against the available certificate chain.
      skip-verify: false
    # integer, defaults to 1000.
    # number of events to buffer before sending a batch to the collector.
    # events are sent every `interval` or when the batch is full, whichever comes first.
    batch-size: 1000
    # duration, defaults to 5s.
    # time interval between export requests.
    interval: 5s
    # integer, defaults to 2x batch-size.
    # size of the internal event buffer.
    buffer-size: 2000
    # integer, defaults to 3.
    # number of retries per export request on failure.
    max-retries: 3
    # string, to be used as the metric namespace
    metric-prefix: ""
    # boolean, if true the subscription name will be prepended to the metric name after the prefix.
    append-subscription-name: false
    # boolean, if true, string type values are exported as gauge metrics with value=1
    # and the string stored as an attribute named "value".
    # if false, string values are dropped.
    strings-as-attributes: false
    # list of tag keys to place as OTLP Resource attributes.
    # these tags are excluded from data point attributes.
    # defaults to empty (all tags become data point attributes).
    resource-tag-keys:
      # - device
      # - vendor
      # - model
      # - site
      # - source
    # list of regex patterns matched against the value key (metric path).
    # if any pattern matches, the metric is exported as a monotonic cumulative Sum (counter).
    # unmatched metrics are exported as Gauges.
    # defaults to empty (all metrics are Gauges).
    counter-patterns:
      # - "counter"
      # - "octets|packets|bytes"
      # - "errors|discards|drops"
    # map of string:string, additional static attributes to add to the OTLP Resource.
    resource-attributes:
      # key: value
    # integer, defaults to 1.
    # number of workers processing events.
    num-workers: 1
    # boolean, defaults to false.
    # enables debug logging.
    debug: false
    # boolean, defaults to false.
    # enables the collection and export (via prometheus) of output specific metrics.
    enable-metrics: false
    # list of processors to apply on the message before writing
    event-processors:
```

## Metric Naming

The metric name is built from up to three parts joined by underscores:

1. The value of `metric-prefix`, if configured.
2. The subscription name, if `append-subscription-name` is `true`.
3. The gNMI path (value key), with `/` and `-` replaced by `_`.

For example, a gNMI update from subscription `port-stats` with path:

```
/interfaces/interface[name=1/1/1]/state/counters/in-octets
```

with `metric-prefix: gnmic` and `append-subscription-name: true`, produces a metric named:

```
gnmic_port_stats_interfaces_interface_state_counters_in_octets
```

## Metric Type Detection

Metrics are classified based on the `counter-patterns` configuration:

- **Sum (monotonic counter)**: if the value key matches any regex in `counter-patterns`.
- **Gauge**: all other numeric values.

By default `counter-patterns` is empty, so all metrics are exported as Gauges. To classify counter-like metrics, configure the patterns explicitly:

```yaml
counter-patterns:
  - "counter|octets|packets|bytes"
  - "errors|discards|drops"
```

Each pattern is a Go [regexp](https://pkg.go.dev/regexp/syntax) matched against the value key (the gNMI path portion of the metric, **before name transformation**).

## Resource and Data Point Attributes

Event tags are split between the OTLP Resource and data point attributes based on the `resource-tag-keys` configuration:

- Tags whose keys appear in `resource-tag-keys` are placed as **Resource attributes** and excluded from data point attributes.
- All remaining tags become **data point attributes** (equivalent to Prometheus labels).

By default `resource-tag-keys` is empty, so all tags become data point attributes. To move device-level metadata to the OTLP Resource (keeping it out of Prometheus labels), configure it explicitly:

```yaml
resource-tag-keys:
  - device
  - vendor
  - model
  - site
  - source
```

Additional static attributes can be added to every Resource using `resource-attributes`:

```yaml
resource-attributes:
  service.name: gnmic-collector
  deployment.environment: production
```

## OTLP Resource Grouping

Events are grouped by their `source` tag (the target device address). Each unique source becomes a separate OTLP `ResourceMetrics` entry with its own set of resource attributes.

## OTLP Output Metrics

When `enable-metrics` is set to `true`, the OTLP output exposes the following Prometheus metrics:

| Metric Name | Type | Description |
|---|---|---|
| `gnmic_otlp_output_number_of_sent_events_total` | Counter | Number of events successfully sent to the OTLP collector |
| `gnmic_otlp_output_number_of_failed_events_total` | Counter | Number of events that failed to send |
| `gnmic_otlp_output_send_duration_seconds` | Histogram | Duration of sending batches to the OTLP collector |
| `gnmic_otlp_output_rejected_data_points_total` | Counter | Number of data points rejected by the collector (PartialSuccess) |
