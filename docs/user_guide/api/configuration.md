
# /api/v1/config

## /api/v1/config

### `GET /api/v1/config`

Request all gnmic configuration

Returns the whole configuration as json

=== "Request"
    ```bash
    curl --request GET gnmic-api-address:port/api/v1/config
    ```
=== "200 OK"
    ```json
    {
        "username": "admin",
        "password": "admin",
        "port": "57400",
        "encoding": "json_ietf",
        "insecure": true,
        "timeout": 10000000000,
        "log": true,
        "max-msg-size": 536870912,
        "prometheus-address": ":8989",
        "retry": 10000000000,
        "api": ":7890",
        "get-type": "ALL",
        "set-delimiter": ":::",
        "subscribe-mode": "stream",
        "subscribe-stream-mode": "target-defined",
        "subscribe-cluster-name": "default-cluster",
        "subscribe-lock-retry": 5000000000,
        "path-path-type": "xpath",
        "prompt-max-suggestions": 10,
        "prompt-prefix-color": "dark_blue",
        "prompt-suggestions-bg-color": "dark_blue",
        "prompt-description-bg-color": "dark_gray",
        "targets": {
            "192.168.1.131:57400": {
                "name": "192.168.1.131:57400",
                "address": "192.168.1.131:57400",
                "username": "admin",
                "password": "admin",
                "timeout": 10000000000,
                "insecure": true,
                "skip-verify": false,
                "buffer-size": 1000,
                "retry-timer": 10000000000
            },
            "192.168.1.132:57400": {
                "name": "192.168.1.132:57400",
                "address": "192.168.1.131:57400",
                "username": "admin",
                "password": "admin",
                "timeout": 10000000000,
                "insecure": true,
                "skip-verify": false,
                "buffer-size": 1000,
                "retry-timer": 10000000000
            }
        },
        "subscriptions": {
            "sub1": {
                "name": "sub1",
                "paths": [
                    "/interface/statistics"
                ],
                "mode": "stream",
                "stream-mode": "sample",
                "encoding": "json_ietf",
                "sample-interval": 1000000000
            }
        },
        "Outputs": {
            "output2": {
                "address": "192.168.1.131:4222",
                "format": "event",
                "subject": "telemetry",
                "type": "nats",
                "write-timeout": "10s"
            }
        },
        "inputs": {},
        "processors": {},
        "clustering": {
            "cluster-name": "cluster1",
            "instance-name": "gnmic1",
            "service-address": "gnmic1",
            "services-watch-timer": 60000000000,
            "targets-watch-timer": 5000000000,
            "leader-wait-timer": 5000000000,
            "locker": {
                "address": "consul-agent:8500",
                "type": "consul"
            }
        }
    }
    ```
=== "500 Internal Server Error"
    ```json
    {
        "errors": [
            "Error Text"
        ]
    }
    ```

## /api/v1/config/targets

### `GET /api/v1/config/targets`

Request all targets configuration

returns the targets configuration as json

=== "Request"
    ```bash
    curl --request GET gnmic-api-address:port/api/v1/config/targets
    ```
=== "200 OK"
    ```json
    {
        "192.168.1.131:57400": {
            "name": "192.168.1.131:57400",
            "address": "192.168.1.131:57400",
            "username": "admin",
            "password": "admin",
            "timeout": 10000000000,
            "insecure": true,
            "skip-verify": false,
            "buffer-size": 1000,
            "retry-timer": 10000000000
        },
        "192.168.1.132:57400": {
            "name": "192.168.1.132:57400",
            "address": "192.168.1.131:57400",
            "username": "admin",
            "password": "admin",
            "timeout": 10000000000,
            "insecure": true,
            "skip-verify": false,
            "buffer-size": 1000,
            "retry-timer": 10000000000
        }
    }
    ```
=== "404 Not found"
    ```json
    {
        "errors": [
            "no targets found",
        ]
    }
    ```
=== "500 Internal Server Error"
    ```json
    {
        "errors": [
            "Error Text"
        ]
    }
    ```

### `GET /api/v1/config/targets/{id}`

Request a single target configuration

Returns a single target configuration as json, where {id} is the target ID

=== "Request"
    ```bash
    curl --request GET gnmic-api-address:port/api/v1/config/targets/192.168.1.131:57400
    ```
=== "200 OK"
    ```json
    {
        "name": "192.168.1.131:57400",
        "address": "192.168.1.131:57400",
        "username": "admin",
        "password": "admin",
        "timeout": 10000000000,
        "insecure": true,
        "skip-verify": false,
        "buffer-size": 1000,
        "retry-timer": 10000000000
    }
    ```
=== "404 Not found"
    ```json
    {
        "errors": [
            "target $target not found",
        ]
    }
    ```
=== "500 Internal Server Error"
    ```json
    {
        "errors": [
            "Error Text"
        ]
    }
    ```

### `POST /api/v1/config/targets`

Add a new target to gnmic configuration

Expected request body is a single target config as json

Returns an empty body if successful.

=== "Request"
    ```bash
    curl --request POST -H "Content-Type: application/json" \
         -d '{"address": "10.10.10.10:57400", "username": "admin", "password": "admin", "insecure": true}' \
         gnmic-api-address:port/api/v1/config/targets
    ```
=== "200 OK"
    ```json
    ```
=== "400 Bad Request"
    ```json
    ```
=== "500 Internal Server Error"
    ```json
    {
        "errors": [
            "Error Text"
        ]
    }
    ```

### `DELETE /api/v1/config/targets/{id}`
  
Deletes a target {id} configuration, all active subscriptions are terminated.

Returns an empty body

=== "Request"
    ```bash
    curl --request DELETE gnmic-api-address:port/api/v1/config/targets/192.168.1.131:57400
    ```
=== "200 OK"
    ```json
    ```

## /api/v1/config/subscriptions

### `GET /api/v1/config/subscriptions`

Request all the configured subscriptions.

Returns the subscriptions configuration as json

## /api/v1/config/outputs

### `GET /api/v1/config/outputs`

Request all the configured outputs.

Returns the outputs configuration as json

## /api/v1/config/inputs

### `GET /api/v1/config/inputs`

Request all the configured inputs.

Returns the outputs configuration as json

## /api/v1/config/processors

### `GET /api/v1/config/processors`

Request all the configured processors.

Returns the processors configuration as json

## /api/v1/config/clustering

### `GET /api/v1/config/clustering`

Request the clustering configuration.

Returns the clustering configuration as json
