# Other

## /api/v1/healthz

### `GET /api/v1/healthz`

Health check endpoint for Kubernetes or similar

=== "Request"
    ```bash
    curl --request GET gnmic-api-address:port/api/v1/healthz
    ```
=== "200 OK"
    ```json
    {
        "status": "healthy"
    }
    ```
    
## /api/v1/admin/shutdown

### `POST /api/v1/admin/shutdown`

Gracefully shut down the application

=== "Request"
    ```bash
    curl --request POST gnmic-api-address:port/api/v1/admin/shutdown
    ```
