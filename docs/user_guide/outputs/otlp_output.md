`gnmic` can export subscription updates as [OpenTelemetry](https://opentelemetry.io/) metrics using the [OpenTelemetry Protocol (OTLP)](https://opentelemetry.io/docs/specs/otlp/).

The OTLP output supports both OTLP/gRPC and OTLP/HTTP. OTLP/gRPC commonly uses port `4317`; OTLP/HTTP commonly uses port `4318` and the `/v1/metrics` path.

## Configuration

### OTLP/gRPC

```yaml
outputs:
  otlp-grpc:
    type: otlp
    endpoint: collector.example.com:4317
    protocol: grpc
```

### OTLP/HTTP

```yaml
outputs:
  otlp-http:
    type: otlp
    endpoint: collector.example.com:4318
    protocol: http
```

With the configuration above, `gnmic` sends OTLP metrics to:

```text
http://collector.example.com:4318/v1/metrics
```

### OTLP/HTTP with mTLS

```yaml
outputs:
  otlp-http:
    type: otlp
    endpoint: collector.example.com:4318
    protocol: http
    tls:
      ca-file: /etc/gnmic/certs/ca.crt
      cert-file: /etc/gnmic/certs/client.crt
      key-file: /etc/gnmic/certs/client.key
```

## Configuration Reference

| Field | Default | Description |
|---|---:|---|
| `type` | required | Must be `otlp`. |
| `endpoint` | required | OTLP endpoint. Bare endpoints must be `host:port`. |
| `protocol` | `grpc` | Transport protocol. One of `grpc` or `http`. |
| `timeout` | `10s` | Timeout for each export attempt. |
| `tls` | unset | TLS configuration. Supports `ca-file`, `cert-file`, `key-file`, and `skip-verify`. |
| `batch-size` | `1000` | Number of events to buffer before sending a batch. |
| `interval` | `5s` | Maximum time to wait before sending a non-empty batch. |
| `buffer-size` | `2 * batch-size` | Size of the internal event channel. Changing it on a live config reload swaps the channel and may drop events still buffered in the old one. |
| `max-retries` | `3` | Number of retry attempts after the first export attempt fails. |
| `compression` | `gzip` for HTTP | HTTP request body compression. One of `gzip` or `none`. Ignored by OTLP/gRPC. |
| `metric-prefix` | unset | Prefix added to generated metric names. |
| `append-subscription-name` | `false` | Adds the subscription name to generated metric names. |
| `strip-leading-underscore` | `false` | Removes a leading `/` from the gNMI path before `/` is converted to `_`. |
| `strings-as-attributes` | `false` | Exports **non-numeric** string values as gauge metrics with value `1` and a `value` data point attribute. Numeric strings (RFC 7951 encodes `uint64`/`int64`/`decimal64` as JSON strings under `json_ietf`) are always emitted as numeric datapoints. If false, non-numeric string values are dropped. |
| `resource-tag-keys` | unset | Tags to place on the OTLP Resource instead of the data point. |
| `counter-patterns` | unset | Regex patterns matched against value names. Matching values are exported as monotonic cumulative Sums. |
| `resource-attributes` | unset | Static attributes added to every OTLP Resource. |
| `headers` | unset | gRPC metadata or HTTP headers added to every export request. |
| `num-workers` | `1` | Number of worker goroutines processing event batches. |
| `debug` | `false` | Enables debug logging for this output. |
| `enable-metrics` | `false` | Enables Prometheus metrics for this output. |
| `event-processors` | unset | Event processors to apply before events are converted to OTLP metrics. |

## Transport

For OTLP/gRPC, bare endpoints must include a port:

```yaml
endpoint: collector.example.com:4317
protocol: grpc
```

gRPC target URIs such as `dns:///collector.example.com:4317` can also be used.

For OTLP/HTTP, the endpoint can be either a bare `host:port` value or a full URL.

Bare HTTP endpoints are converted to a full metrics URL. Without `tls`, `gnmic` uses `http`; with `tls`, it uses `https`.

```yaml
endpoint: collector.example.com:4318
protocol: http
```

This becomes:

```text
http://collector.example.com:4318/v1/metrics
```

Full HTTP URLs preserve the configured scheme and path:

```yaml
endpoint: https://collector.example.com/custom/metrics/path
protocol: http
```

If the path is empty or `/`, it defaults to `/v1/metrics`.

Only `http` and `https` URL schemes are accepted for OTLP/HTTP. User information in URLs is rejected; use `headers` for authentication data.

HTTP redirects are never followed: a redirect response is treated as a permanent export failure. This prevents a redirecting endpoint from receiving the metrics payload and configured headers at a different origin.

## TLS and Compression

The `tls` block applies to both OTLP/gRPC and OTLP/HTTP.

For OTLP/HTTP, do not configure an `http://` URL with a `tls` block. Use a bare `host:port` endpoint or an `https://` URL when TLS is required.

When `protocol: http` is used, request body compression defaults to `gzip`:

```yaml
outputs:
  otlp-http:
    type: otlp
    endpoint: collector.example.com:4318
    protocol: http
    compression: gzip
```

Set `compression: none` to disable HTTP request compression. The `compression` field is ignored by OTLP/gRPC.

## Metric Names

Metric names are built from:

1. `metric-prefix`, if configured.
2. The subscription name, if `append-subscription-name` is `true`.
3. The gNMI value path, with `/` and `-` replaced by `_`.

For example, given a subscription named `port-stats` defined under the `subscriptions:` section of the `gnmic` configuration:

```yaml
subscriptions:
  port-stats:
    paths:
      - /interfaces/interface/state/counters/in-octets
    mode: stream
```

a gNMI update from this subscription with path:

```text
/interfaces/interface[name=1/1/1]/state/counters/in-octets
```

with `metric-prefix: gnmic` and `append-subscription-name: true`, produces:

```text
gnmic_port_stats__interfaces_interface_state_counters_in_octets
```

The `port_stats` segment is the subscription name `port-stats` after `-` is converted to `_` (Prometheus does not allow hyphens in metric names).

The leading `/` of the gNMI path becomes a leading `_` in the path-derived segment, which combines with the trailing `_` of the prefix or subscription name to form a `__` separator. Set `strip-leading-underscore: true` to elide the leading `/` and produce a single-`_` separator:

```text
gnmic_port_stats_interfaces_interface_state_counters_in_octets
```

With `strip-leading-underscore: true`, a value key that begins with `/` has that slash removed before `/` is converted to `_`, so you do not get an extra `_` at the join between the configured prefixes and the path-derived segment (absolute paths otherwise contribute a leading `_` from the first `/`).

Boolean gNMI values are exported as numeric data points with `0` or `1` (integer), classified as a Gauge or Sum according to `counter-patterns` like other numeric types.

## Metric Types

By default, numeric values are exported as Gauges.

Use `counter-patterns` to export matching values as monotonic cumulative Sums:

```yaml
counter-patterns:
  - "counter|octets|packets|bytes"
  - "errors|discards|drops"
```

Patterns are Go [regexp](https://pkg.go.dev/regexp/syntax) expressions matched against the value name before metric name transformation.

## Resource Attributes

Event tags are exported as data point attributes by default.

Use `resource-tag-keys` to move selected tags to the OTLP Resource:

```yaml
resource-tag-keys:
  - device
  - vendor
  - model
  - site
  - source
```

Static Resource attributes can be configured with `resource-attributes`:

```yaml
resource-attributes:
  service.name: gnmic
  deployment.environment: production
```

Events are grouped by a stable identifier of the producing entity — the first non-empty tag in the chain `device → target → source`, or `unknown` if all are absent. Each group becomes one OTLP `ResourceMetrics` entry.

## Headers

The `headers` field attaches key/value pairs to every export request. Headers are sent as gRPC metadata when `protocol: grpc` is used, and as HTTP headers when `protocol: http` is used.

```yaml
outputs:
  otlp-http:
    type: otlp
    endpoint: collector.example.com:4318
    protocol: http
    headers:
      X-Scope-OrgID: my-tenant
      Authorization: Bearer <token>
```

For OTLP/HTTP, user-supplied headers cannot override the required OTLP `Content-Type` or `Content-Encoding` headers.

Header values are treated as credentials: `gnmic` redacts them in its own log output (header names remain visible).

## Retries and Partial Success

`max-retries` applies to both OTLP/gRPC and OTLP/HTTP.

For OTLP/HTTP, retryable status codes are `429`, `502`, `503`, and `504`. Other HTTP status codes are treated as permanent failures. When a retryable response includes a `Retry-After` header, `gnmic` honors it up to an internal cap of 30 seconds; otherwise it uses exponential backoff with jitter.

OTLP `PartialSuccess` responses with rejected data points are not retried for either OTLP/gRPC or OTLP/HTTP. When output metrics are enabled, rejected data points are counted by `gnmic_otlp_output_rejected_data_points_total`.

A `PartialSuccess` response counts as a delivered export: the batch is accounted in `gnmic_otlp_output_number_of_sent_events_total` (the sent/failed counters track export requests at event granularity), while the partial loss is reported exclusively by `gnmic_otlp_output_rejected_data_points_total`. Alert on the rejected counter to detect data loss — a batch whose data points are all rejected still counts as sent.

## Output Metrics

When `enable-metrics` is set to `true`, the OTLP output exposes the following Prometheus metrics:

| Metric Name | Type | Description |
|---|---|---|
| `gnmic_otlp_output_number_of_sent_events_total` | Counter | Number of events successfully sent to the OTLP endpoint. |
| `gnmic_otlp_output_number_of_failed_events_total` | Counter | Number of events that failed to send. |
| `gnmic_otlp_output_send_duration_seconds` | Histogram | Duration of sending batches to the OTLP endpoint. |
| `gnmic_otlp_output_rejected_data_points_total` | Counter | Number of data points rejected by the OTLP endpoint in `PartialSuccess` responses. |
| `gnmic_otlp_output_malformed_responses_total` | Counter | Number of successful (2xx) OTLP/HTTP export responses whose body could not be parsed as an OTLP response. |
