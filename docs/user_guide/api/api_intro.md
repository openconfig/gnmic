A limited set of REST endpoints are supported, these are mainly used to allow for a clustered deployment for multiple `gnmic` instances.

The API can be used to automate (to a certain extent) the targets configuration loading and starting/stopping subscriptions.

## Configuration

Enabling the API server can be done via a command line flag:

```bash
gnmic --config gnmic.yaml subscribe --api ":7890"
```

via ENV variable: `GNMIC_API=':7890'`

Or via file configuration, by adding the below line to the config file:

```yaml
api: ":7890"
```

More advanced API configuration options (like a secure API Server)
can be achieved by setting the fields under `api-server`.

```yaml
api-server:
  # string, in the form IP:port, the IP part can be omitted.
  # if not set, it defaults to the value of `api` in the file main level.
  # if `api` is not set, the default is `:7890`
  address: :7890
  # duration, the server timeout.
  # The set value is equally split between read and write timeouts
  timeout: 10s
  # boolean, if true, the server will not verify the client's certificates
  skip-verify: false
  # path to the CA certificate file to be used, 
  # irrelevant if `skip-verify` is true
  ca-file: 
  # path to the server certificate file
  cert-file:
  # path to the server key file
  key-file:
  # boolean, if true, the server will also handle the path /metrics and serve 
  # gNMIc's enabled prometheus metrics.
  enable-metrics: false
  # boolean, enables extra debug log printing
  debug: false
```

## API Endpoints

* [Configuration](./configuration.md)

* [Targets](./targets.md)

* [Cluster](./cluster.md)
