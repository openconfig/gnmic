`gnmic` can export subscription updates to [ClickHouse](https://clickhouse.com/) using the native protocol (`clickhouse-go`). Rows are derived from the same [event](../event_processors/intro.md#the-event-format) pipeline as other database outputs: each path/value (or delete) becomes one row in a typed telemetry table.

## Configuration

Define a ClickHouse output under `outputs` with `type: clickhouse`:

```yaml
outputs:
  clickhouse-telemetry:
    type: clickhouse
    # ClickHouse native TCP address (host:port)
    address: localhost:9000
    # Database and table (identifiers: letters, digits, underscore only)
    database: default
    table: gnmic_telemetry
    username: default
    password: ""
    # TLS: set this block to enable TLS.
    # For TLS without CA material, use skip-verify: true.
    tls:
      ca-file:
      cert-file:
      key-file:
      skip-verify: false
    # If true, CREATE TABLE IF NOT EXISTS is run when the connection is ready (default: true).
    create-table: true
    # MergeTree DDL knobs (defaults match the built-in DDL)
    table-engine: MergeTree
    partition-by: toYYYYMMDD(timestamp)
    order-by: ["target", "path", "timestamp"]
    # TTL on the MergeTree table. Shorthand like "30 DAY" is expanded for ClickHouse.
    # Omit from config for default retention; set ttl: "" explicitly in YAML to disable TTL.
    ttl: 30 DAY
    # Batching and concurrency
    batch-size: 1000
    flush-timer: 5s
    num-workers: 1
    buffer-size: 10000
    max-in-flight: 1
    timeout: 5s
    recovery-wait-time: 10s
    # If true, sample timestamps are replaced with export time
    override-timestamps: false
    # add-target / target-template
    add-target: ""
    target-template: ""
    # Processors applied before row conversion
    event-processors: []
    debug: false
    enable-metrics: false
```

### Defaults

| Field | Default |
|-------|---------|
| `address` | `localhost:9000` |
| `database` | `default` |
| `table` | `gnmic_telemetry` |
| `username` | `default` |
| `batch-size` | `1000` |
| `flush-timer` | `5s` |
| `timeout` | `5s` |
| `recovery-wait-time` | `10s` |
| `num-workers` | `1` |
| `buffer-size` | `10000` |
| `max-in-flight` | `1` |
| `table-engine` | `MergeTree` |
| `partition-by` | `toYYYYMMDD(timestamp)` |
| `order-by` | `["target", "path", "timestamp"]` |
| `ttl` | `30 DAY` (unless `ttl` is set to empty in YAML to disable) |
| `create-table` | `true` (unless `create-table` appears in YAML) |

TLS is enabled only when the `tls:` block is present. Use `tls.skip-verify: true` for encrypted connections without local CA files (equivalent to “insecure” client mode in other stacks).

## Table layout

When `create-table: true`, `gnmic` creates a MergeTree table roughly equivalent to:

| Column | Purpose |
|--------|---------|
| `timestamp` | Sample time (`DateTime64(9,'UTC')`) |
| `ingest_ts` | Insert time (`DateTime64(9,'UTC')`, default `now64(9)`) |
| `target`, `source`, `subscription`, `name` | Dimensions |
| `path` | gNMI path string |
| `tags` | `Map(LowCardinality(String), String)` from event tags |
| `value_type` | `int`, `uint`, `float`, `bool`, `string`, `bytes`, `json`, … |
| `value_int`, `value_uint`, `value_float`, `value_bool` | Typed nullable columns (only one meaningful for a row) |
| `value_string` | String / JSON / base64 for bytes |
| `is_delete` | `true` when the row represents a path delete |

You can add **regular views** for ad hoc projections (no storage):

```sql
CREATE VIEW IF NOT EXISTS default.v_latest_strings AS
SELECT timestamp, target, path, value_string, tags
FROM default.gnmic_telemetry
WHERE value_type = 'string' AND NOT is_delete;
```

## Materialized views

A **materialized view** in ClickHouse runs on each insert into the source table and writes its `SELECT` result into a **target** table (`TO target_table`). That matches how `gnmic` streams inserts.

!!! warning "Aggregations in materialized views"
    A `GROUP BY` inside a materialized view only sees **each incoming insert block**, not the full history of `gnmic_telemetry`. Use materialized views without `GROUP BY` for row-wise transforms, or use [Refreshable Materialized Views](https://clickhouse.com/docs/en/sql-reference/statements/create/view#refreshable-materialized-view) / scheduled jobs for full-table rollups.

### Example 1: Narrow table (filtered copy)

Useful for smaller, faster scans (e.g. only interface counters):

```sql
CREATE TABLE IF NOT EXISTS default.iface_counters
(
    timestamp DateTime64(9, 'UTC'),
    ingest_ts   DateTime64(9, 'UTC'),
    target      LowCardinality(String),
    path        String,
    value_uint  Nullable(UInt64),
    tags        Map(LowCardinality(String), String)
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (target, path, timestamp);

CREATE MATERIALIZED VIEW IF NOT EXISTS default.mv_iface_counters
TO default.iface_counters
AS
SELECT
    timestamp,
    ingest_ts,
    target,
    path,
    value_uint,
    tags
FROM default.gnmic_telemetry
WHERE startsWith(path, '/interface/')
  AND NOT is_delete
  AND value_type = 'uint';
```

### Example 2: Last string value per `(target, path)` (ReplacingMergeTree)

Keeps the newest row per key using `ingest_ts` as the version column:

```sql
CREATE TABLE IF NOT EXISTS default.path_latest_string
(
    target      LowCardinality(String),
    path        String,
    timestamp   DateTime64(9, 'UTC'),
    value_string String,
    ingest_ts   DateTime64(9, 'UTC')
)
ENGINE = ReplacingMergeTree(ingest_ts)
ORDER BY (target, path);

CREATE MATERIALIZED VIEW IF NOT EXISTS default.mv_path_latest_string
TO default.path_latest_string
AS
SELECT
    target,
    path,
    timestamp,
    value_string,
    ingest_ts
FROM default.gnmic_telemetry
WHERE value_type = 'string'
  AND NOT is_delete;
```

Query with `FINAL` or rely on background merges depending on your consistency needs.

### Example 3: Denormalized “value text” column

Single column for dashboards that do not want to branch on `value_type`:

```sql
CREATE TABLE IF NOT EXISTS default.gnmic_flat_values
(
    timestamp   DateTime64(9, 'UTC'),
    target      LowCardinality(String),
    path        String,
    value_text  String,
    value_type  LowCardinality(String),
    tags        Map(LowCardinality(String), String),
    ingest_ts   DateTime64(9, 'UTC')
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (target, path, timestamp);

CREATE MATERIALIZED VIEW IF NOT EXISTS default.mv_gnmic_flat_values
TO default.gnmic_flat_values
AS
SELECT
    timestamp,
    target,
    path,
    multiIf(
        value_type = 'int',    toString(value_int),
        value_type = 'uint',   toString(value_uint),
        value_type = 'float',  toString(value_float),
        value_type = 'bool',   toString(value_bool),
        value_string
    ) AS value_text,
    value_type,
    tags,
    ingest_ts
FROM default.gnmic_telemetry
WHERE NOT is_delete;
```

### Backfilling

Materialized views only process **new** inserts. To populate a target table from existing data:

```sql
INSERT INTO default.iface_counters
SELECT timestamp, ingest_ts, target, path, value_uint, tags
FROM default.gnmic_telemetry
WHERE startsWith(path, '/interface/')
  AND NOT is_delete
  AND value_type = 'uint';
```

## Metrics

When `enable-metrics: true`, the output registers Prometheus metrics (received/inserted/failed/dropped rows, batch size, insert latency, buffer length). A Prometheus registry must be available (same pattern as other outputs that integrate with the main `gnmic` metrics server).

## See also

* [Event format and processors](../event_processors/intro.md)
* Example lab: [ClickHouse output deployment](../../deployments/single-instance/containerlab/clickhouse-output.md) (`examples/deployments/1.single-instance/12.clickhouse-output/containerlab/`)
