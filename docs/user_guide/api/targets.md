## `GET /api/v1/targets`

Request all active targets details.

Returns all active targets as json.

By default, target **`password`** and **`token`** in each `config` object are redacted to **`****`**. To include real values, set `expose-target-secrets: true` under `api-server` in your configuration (use only on trusted networks).

=== "Request"
    ```bash
    curl --request GET gnmic-api-address:port/api/v1/targets
    ```
=== "200 OK"
    ```json
    {
        "192.168.1.131:57400": {
            "config": {
                "name": "192.168.1.131:57400",
                "address": "192.168.1.131:57400",
                "username": "admin",
                "password": "****",
                "timeout": 10000000000,
                "insecure": true,
                "skip-verify": false,
                "buffer-size": 1000,
                "retry-timer": 10000000000
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
            }
        },
        "192.168.1.131:57401": {
            "config": {
                "name": "192.168.1.131:57401",
                "address": "192.168.1.131:57401",
                "username": "admin",
                "password": "****",
                "timeout": 10000000000,
                "insecure": true,
                "skip-verify": false,
                "buffer-size": 1000,
                "retry-timer": 10000000000
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
            }
        }
    }
    ```
=== "404 Not found"
    ```json
    {
        "errors": [
            "no targets found"
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

## `GET /api/v1/targets/{id}`

Query a single target details, if active.

Returns a single target if active as json, where {id} is the target ID. Values of `config.password` and `config.token` follow the same redaction behavior as the list-all-targets endpoint above.

=== "Request"
    ```bash
    curl --request GET gnmic-api-address:port/api/v1/targets/192.168.1.131:57400
    ```
=== "200 OK"
    ```json
    {
        "config": {
            "name": "192.168.1.131:57400",
            "address": "192.168.1.131:57400",
            "username": "admin",
            "password": "****",
            "timeout": 10000000000,
            "insecure": true,
            "skip-verify": false,
            "buffer-size": 1000,
            "retry-timer": 10000000000
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
        }
    }
    ```
=== "404 Not found"
    ```json
    {
        "errors": [
            "no targets found"
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

## `POST /api/v1/targets/{id}`

Starts a single target subscriptions, where {id} is the target ID

Returns an empty body if successful.

=== "Request"
    ```bash
    curl --request POST gnmic-api-address:port/api/v1/targets/192.168.1.131:57400
    ```
=== "200 OK"
    ```json
    ```
=== "404 Not found"
    ```json
    {
        "errors": [
            "target $target not found"
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

## `DELETE /api/v1/targets/{id}`
  
Stops a single target active subscriptions, where {id} is the target ID
    
Returns an empty body if successful.

=== "Request"
    ```bash
    curl --request DELETE gnmic-api-address:port/api/v1/targets/192.168.1.131:57400
    ```
=== "200 OK"
    ```json
    ```
=== "404 Not found"
    ```json
    {
        "errors": [
            "target $target not found"
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

