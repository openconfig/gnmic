# Collector REST API

The collector exposes a REST API for dynamic configuration management and status queries. This API is specific to the collector mode and differs from the API available in subscribe mode.

## Base URL

All API endpoints are prefixed with `/api/v1`. For example, if the API server is running on `localhost:7890`:

```
http://localhost:7890/api/v1/targets
```

## Authentication

If TLS is configured with client authentication, requests must include valid client certificates.

## Common Response Formats

### Success Response

Most successful responses return JSON with HTTP status 200.

### Error Response

Error responses include an `errors` array:

```json
{
  "errors": ["error message 1", "error message 2"]
}
```

---

## Health & Admin Endpoints

### Health Check

```
GET /api/v1/healthz
```

Returns the health status of the collector.

**Response:** `200 OK` if healthy

### Shutdown

```
POST /api/v1/admin/shutdown
```

Not implemented in Collector mode

---

## Configuration Endpoints

### Get Full Configuration

```
GET /api/v1/config
```

Returns the current configuration of the collector.

### Apply Configuration

```
POST /api/v1/config/apply
```

Applies a complete configuration to the collector. Resources not included in the request are deleted.

**Request Body:**

```json
{
  "targets": {
    "router1": {
      "address": "10.0.0.1:57400",
      "username": "admin",
      "password": "admin",
      "skip-verify": true,
      "subscriptions": ["interfaces"]
    }
  },
  "subscriptions": {
    "interfaces": {
      "paths": ["/interfaces/interface/state/counters"],
      "mode": "stream",
      "stream-mode": "sample",
      "sample-interval": "10s"
    }
  },
  "outputs": {
    "prometheus": {
      "type": "prometheus",
      "listen": ":9804"
    }
  },
  "inputs": {},
  "processors": {},
  "tunnel-target-matches": {}
}
```

**Validation Rules:**

- If `targets` are provided, at least one `subscription` is required
- If `inputs` are provided, at least one `output` is required
- Empty request is valid (resets all configuration)

**Headers:**

- `Content-Encoding: gzip` - Request body is gzip compressed

---

## Targets

### List Targets (Runtime State)

```
GET /api/v1/targets
```

Returns all targets with their runtime state (connection status, active subscriptions).

**Response:**

```json
[
  {
    "name": "router1",
    "state": "running",
    "config": {
      "address": "10.0.0.1:57400",
      "username": "admin",
      "skip-verify": true
    },
    "subscriptions": {
      "interfaces": {
        "state": "running"
      }
    }
  }
]
```

### Get Target (Runtime State)

```
GET /api/v1/targets/{name}
```

Returns a specific target with its runtime state.

### List Target Configurations

```
GET /api/v1/config/targets
```

Returns target configurations (without runtime state).

### Get Target Configuration

```
GET /api/v1/config/targets/{name}
```

### Create/Update Target

```
POST /api/v1/config/targets
```

**Request Body:**

```json
{
  "name": "router1",
  "address": "10.0.0.1:57400",
  "username": "admin",
  "password": "admin",
  "skip-verify": true,
  "subscriptions": ["interfaces"],
  "outputs": ["prometheus"]
}
```

### Delete Target

```
DELETE /api/v1/config/targets/{name}
```

### Update Target Subscriptions

```
PATCH /api/v1/config/targets/{name}/subscriptions
```

**Request Body:**

```json
{
  "subscriptions": ["interfaces", "bgp"]
}
```

### Update Target Outputs

```
PATCH /api/v1/config/targets/{name}/outputs
```

**Request Body:**

```json
{
  "outputs": ["prometheus", "influxdb"]
}
```

### Update Target State

```
POST /api/v1/config/targets/{name}/state
POST /api/v1/targets/{name}/state/{state}
```

Enable or disable a target. State can be `enabled` or `disabled`.

---

## Subscriptions

### List Subscriptions (Runtime State)

```
GET /api/v1/subscriptions
```

Returns subscriptions with their runtime state (which targets are using them).

**Response:**

```json
[
  {
    "name": "interfaces",
    "config": {
      "paths": ["/interfaces/interface/state/counters"],
      "mode": "stream",
      "stream-mode": "sample",
      "sample-interval": "10s"
    },
    "targets": {
      "router1": {
        "state": "running"
      }
    }
  }
]
```

### Get Subscription (Runtime State)

```
GET /api/v1/subscriptions/{name}
```

### List Subscription Configurations

```
GET /api/v1/config/subscriptions
```

### Get Subscription Configuration

```
GET /api/v1/config/subscriptions/{name}
```

### Create/Update Subscription

```
POST /api/v1/config/subscriptions
```

**Request Body:**

```json
{
  "name": "interfaces",
  "paths": ["/interfaces/interface/state/counters"],
  "mode": "stream",
  "stream-mode": "sample",
  "sample-interval": "10s",
  "encoding": "json",
  "outputs": ["prometheus"]
}
```

### Delete Subscription

```
DELETE /api/v1/config/subscriptions/{name}
```

---

## Outputs

### List Output Configurations

```
GET /api/v1/config/outputs
```

**Response:**

```json
{
  "prometheus": {
    "type": "prometheus",
    "listen": ":9804",
    "path": "/metrics"
  }
}
```

### Get Output Configuration

```
GET /api/v1/config/outputs/{name}
```

### Create/Update Output

```
POST /api/v1/config/outputs
```

**Request Body:**

```json
{
  "name": "prometheus",
  "type": "prometheus",
  "listen": ":9804",
  "path": "/metrics",
  "event-processors": ["trim-prefixes"]
}
```

### Delete Output

```
DELETE /api/v1/config/outputs/{name}
```

### Update Output Processors

```
PATCH /api/v1/config/outputs/{name}/processors
```

**Request Body:**

```json
{
  "event-processors": ["processor1", "processor2"]
}
```

**Note:** Currently returns `501 Not Implemented`.

---

## Inputs

### List Input Configurations

```
GET /api/v1/config/inputs
```

**Response:**

```json
{
  "nats-input": {
    "type": "nats",
    "address": "nats://localhost:4222",
    "subject": "telemetry.>"
  }
}
```

### Get Input Configuration

```
GET /api/v1/config/inputs/{name}
```

### Create/Update Input

```
POST /api/v1/config/inputs
```

**Request Body:**

```json
{
  "name": "nats-input",
  "type": "nats",
  "address": "nats://localhost:4222",
  "subject": "telemetry.>",
  "outputs": ["prometheus"],
  "event-processors": ["add-tags"]
}
```

### Delete Input

```
DELETE /api/v1/config/inputs/{name}
```

### Update Input Processors

```
PATCH /api/v1/config/inputs/{name}/processors
```

**Note:** Currently returns `501 Not Implemented`.

### Update Input Outputs

```
PATCH /api/v1/config/inputs/{name}/outputs
```

**Note:** Currently returns `501 Not Implemented`.

---

## Processors

### List Processor Configurations

```
GET /api/v1/config/processors
```

**Response:**

```json
[
  {
    "name": "trim-prefixes",
    "type": "event-strings",
    "config": {
      "value-names": [".*"],
      "transforms": [...]
    }
  }
]
```

### Get Processor Configuration

```
GET /api/v1/config/processors/{name}
```

### Create/Update Processor

```
POST /api/v1/config/processors
```

### Delete Processor

```
DELETE /api/v1/config/processors/{name}
```

---

## Tunnel Target Matches

### List Tunnel Target Matches

```
GET /api/v1/config/tunnel-target-matches
```

### Get Tunnel Target Match

```
GET /api/v1/config/tunnel-target-matches/{name}
```

### Create/Update Tunnel Target Match

```
POST /api/v1/config/tunnel-target-matches
```

**Request Body:**

```json
{
  "name": "srl-devices",
  "target-type": "srlinux",
  "subscriptions": ["interfaces"],
  "outputs": ["prometheus"]
}
```

### Delete Tunnel Target Match

```
DELETE /api/v1/config/tunnel-target-matches/{name}
```

---

## Cluster Endpoints

### Get Cluster Status

```
GET /api/v1/cluster
```

Returns the current cluster status including membership and target distribution.

### Get Leader

```
GET /api/v1/cluster/leader
```

Returns information about the current cluster leader.

### Release Leadership

```
DELETE /api/v1/cluster/leader
```

Forces the current leader to release leadership (triggers new election).

### Get Members

```
GET /api/v1/cluster/members
```

Returns list of cluster members with their status.

### Drain Instance

```
POST /api/v1/cluster/members/{id}/drain
```

Drains all targets from a specific instance (moves them to other instances).

### Rebalance

```
POST /api/v1/cluster/rebalance
```

Triggers a rebalance of targets across cluster members.

### Move Target

```
POST /api/v1/cluster/move
```

Moves a specific target to a different instance.

**Request Body:**

```json
{
  "target": "router1",
  "instance": "collector-2"
}
```

---

## Assignments

### List Assignments

```
GET /api/v1/assignments
```

Returns current target-to-instance assignments.

### Get Assignment

```
GET /api/v1/assignments/{target}
```

### Create Assignment

```
POST /api/v1/assignments
```

Manually assign a target to an instance.

### Delete Assignment

```
DELETE /api/v1/assignments/{target}
```

---

## Metrics

```
GET /metrics
```

Returns Prometheus metrics for the collector (if `enable-metrics: true` in api-server config).

---

## Examples

### Using curl

```bash
# List all targets
curl http://localhost:7890/api/v1/targets

# Create a target
curl -X POST http://localhost:7890/api/v1/config/targets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "router1",
    "address": "10.0.0.1:57400",
    "username": "admin",
    "password": "admin",
    "skip-verify": true,
    "subscriptions": ["interfaces"]
  }'

# Delete a target
curl -X DELETE http://localhost:7890/api/v1/config/targets/router1

# Apply full configuration
curl -X POST http://localhost:7890/api/v1/config/apply \
  -H "Content-Type: application/json" \
  -d @config.json

# Apply gzipped configuration
curl -X POST http://localhost:7890/api/v1/config/apply \
  -H "Content-Type: application/json" \
  -H "Content-Encoding: gzip" \
  --data-binary @config.json.gz
```

### Using gnmic CLI

The collector subcommands use the same API endpoints:

```bash
# Uses GET /api/v1/targets
gnmic --config collector.yaml collect targets list

# Uses GET /api/v1/targets/{name}
gnmic --config collector.yaml collect targets get --name router1

# Uses POST /api/v1/config/targets
gnmic --config collector.yaml collect targets set --input target.yaml

# Uses DELETE /api/v1/config/targets/{name}
gnmic --config collector.yaml collect targets delete --name router1
```
