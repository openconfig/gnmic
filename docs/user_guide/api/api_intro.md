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
  # tls config
  tls:
    # string, path to the CA certificate file,
    # this certificate is used to verify the clients certificates.
    ca-file:
    # string, server certificate file.
    cert-file:
    # string, server key file.
    key-file:
    # string, one of `"", "request", "require", "verify-if-given", or "require-verify" 
    #  - request:         The server requests a certificate from the client but does not 
    #                     require the client to send a certificate. 
    #                     If the client sends a certificate, it is not required to be valid.
    #  - require:         The server requires the client to send a certificate and does not 
    #                     fail if the client certificate is not valid.
    #  - verify-if-given: The server requests a certificate, 
    #                     does not fail if no certificate is sent. 
    #                     If a certificate is sent it is required to be valid.
    #  - require-verify:  The server requires the client to send a valid certificate.
    #
    # if no ca-file is present, `client-auth` defaults to ""`
    # if a ca-file is set, `client-auth` defaults to "require-verify"`
    client-auth: ""
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

* [Other](./other.md)
