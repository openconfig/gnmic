`gnmic` can be configured using environment variables, it will read the environment variables starting with `GNMIC_`.

The Env variable names are inline with the flag names as well as the configuration hierarchy.

For e.g to set the gNMI username, the env variable `GNMIC_USERNAME` should be set.

### Constructing environment variables names

#### Flags to environment variables mapping

Global flags to env variable name mapping:

| **Flag name**        | **ENV variable name**    |
| -------------------- | ------------------------ |
| --address            | GNMIC_ADDRESS            |
| --encoding           | GNMIC_ENCODING           |
| --format             | GNMIC_FORMAT             |
| --insecure           | GNMIC_INSECURE           |
| --log                | GNMIC_LOG                |
| --log-file           | GNMIC_LOG_FILE           |
| --no-prefix          | GNMIC_NO_PREFIX          |
| --password           | GNMIC_PASSWORD           |
| --prometheus-address | GNMIC_PROMETHEUS_ADDRESS |
| --proxy-from-env     | GNMIC_PROXY_FROM_ENV     |
| --retry              | GNMIC_RETRY              |
| --skip-verify        | GNMIC_SKIP_VERIFY        |
| --timeout            | GNMIC_TIMEOUT            |
| --tls-ca             | GNMIC_TLS_CA             |
| --tls-cert           | GNMIC_TLS_CERT           |
| --tls-key            | GNMIC_TLS_KEY            |
| --tls-max-version    | GNMIC_TLS_MAX_VERSION    |
| --tls-min-version    | GNMIC_TLS_MIN_VERSION    |
| --tls-version        | GNMIC_TLS_VERSION        |
| --log-tls-secret     | GNMIC_LOG_TLS_SECRET     |
| --username           | GNMIC_USERNAME           |
| --cluster-name       | GNMIC_CLUSTER_NAME       |
| --instance-name      | GNMIC_INSTANCE_NAME      |
| --proto-file         | GNMIC_PROTO_FILE         |
| --proto-dir          | GNMIC_PROTO_DIR          |
| --token              | GNMIC_TOKEN              |

#### Configuration file to environment variables mapping

For configuration items that do not have a corresponding flag, the env variable will be constructed from the path elements to the variable name joined with a `_`.

For e.g to set the clustering locker address, as in the yaml blob below:

```yaml
clustering:
  locker:
    address: 
```

the env variable `GNMIC_CLUSTERING_LOCKER_ADDRESS` should be set

!!! note

    - Configuration items of type list cannot be set using env vars.
    - Intermediate configuration keys should not contain `_` or `-`.

Example:

```yaml
outputs:
  output1:  # <-- should not contain `_` or `-`
    type: prometheus
    listen: :9804
```

Is equivalent to:  
`GNMIC_OUTPUTS_OUTPUT1_TYPE=prometheus`  
`GNMIC_OUTPUTS_OUTPUT1_LISTEN=:9804`
