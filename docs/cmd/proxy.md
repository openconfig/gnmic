### Description

The `[proxy]` command start a gNMI proxy server. That relays gNMI messages to know targets (either configured or discovered).

`gNMIc` proxy relays `Get`, `Set` and `Subscribe` RPCs but not `Capabilities`.

To designate the target of an RPC, the `Prefix.Target` field within the RPC request message should be utilized. This field is versatile, accepting a single target, a comma-separated list of targets, or the wildcard character `*` for broader targeting.

Here are the key points regarding target specification:

- The target can be set to a target name or a comma-separated list of targets.
- Setting the target to `*` implies the selection of all known targets.
- If the Prefix.Target field is not explicitly set, gNMIc defaults to treating it as if `*` were specified, thus applying the action to all known targets.

gNMIc optimizes resource usage by reusing existing gNMI client instances whenever possible. If an appropriate gNMI client does not already exist, gNMIc will create a new instance as required.

### Usage

`gnmic [global-flags] proxy`

### Configuration

The Proxy behavior is controlled using the `gnmi-server` section of the main config file:

```yaml
gnmi-server:
  # the address the gNMI server will listen to
  address: :57400
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
  max-subscriptions: 64
  # maximum number of active Get/Set RPCs
  max-unary-rpc: 64
  # defines the maximum msg size (in bytes) the server can receive, 
  # defaults to 4MB
  max-recv-msg-size:
  # defines the maximum msg size (in bytes) the server can send,
  # default to MaxInt32 (2147483647 bytes or 2.147483647 Gb)
  max-send-msg-size:
  # defines the maximum number of streams per streaming RPC.
  max-concurrent-streams:
  # defines the TCP keepalive tiem and interval for client connections, 
  # if unset it is enabled based on the OS. If negative it is disabled.
  tcp-keepalive: 
  # set keepalive and max-age parameters on the server-side.
  keepalive:
    # MaxConnectionIdle is a duration for the amount of time after which an
    # idle connection would be closed by sending a GoAway. Idleness duration is
    # defined since the most recent time the number of outstanding RPCs became
    # zero or the connection establishment.
    # The current default value is infinity.
    max-connection-idle:
    # MaxConnectionAge is a duration for the maximum amount of time a
    # connection may exist before it will be closed by sending a GoAway. A
    # random jitter of +/-10% will be added to MaxConnectionAge to spread out
    # connection storms.
    # The current default value is infinity.
    max-connection-age:
    # MaxConnectionAgeGrace is an additive period after MaxConnectionAge after
    # which the connection will be forcibly closed.
    # The current default value is infinity.
    max-connection-age-grace:
    # After a duration of this time if the server doesn't see any activity it
    # pings the client to see if the transport is still alive.
    # If set below 1s, a minimum value of 1s will be used instead.
    # The current default value is 2 hours.
    time: 120m
    # After having pinged for keepalive check, the server waits for a duration
    # of Timeout and if no activity is seen even after that the connection is
    # closed.
    # The current default value is 20 seconds.
    timeout: 20s
  # defines the minimum allowed sample interval, this value is used when the received sample-interval 
  # is greater than zero but lower than this minimum value.
  min-sample-interval: 1ms
  # defines the default sample interval, 
  # this value is used when the received sample-interval is zero within a stream/sample subscription.
  default-sample-interval: 1s
  # defines the minimum heartbeat-interval
  # this value is used when the received heartbeat-interval is greater than zero but
  # lower than this minimum value
  min-heartbeat-interval: 1s
  # enables the collection of Prometheus gRPC server metrics
  enable-metrics: false
  # enable additional debug logs
  debug: false
  # Enables Consul service registration
  service-registration:
    # Consul server address, default to localhost:8500
    address:
    # Consul Data center, defaults to dc1
    datacenter: 
    # Consul username, to be used as part of HTTP basicAuth
    username:
    # Consul password, to be used as part of HTTP basicAuth
    password:
    # Consul Token, is used to provide a per-request ACL token 
    # which overrides the agent's default token
    token:
    # gnmi server service check interval, only TTL Consul check is enabled
    # defaults to 5s
    check-interval:
    # Maximum number of failed checks before the service is deleted by Consul
    # defaults to 3
    max-fail:
    # Consul service name
    name:
    # List of tags to be added to the service registration, 
    # if available, the instance-name and cluster-name will be added as tags,
    # in the format: gnmic-instance=$instance-name and gnmic-cluster=$cluster-name
    tags:
```

### Example

#### simple proxy

This config start gNMIc as a gNMI proxy serving 2 targets `router1` and `router2`

```yaml
gnmi-server:
  address: :57401

targets:
  router1:
    skip-verify: true
  router2:
    skip-verify: true
```

```shell
gnmic --config gnmic.yaml proxy
```

#### proxy with target discovery

```yaml
gnmi-server:
  address: :57401

loader:
  type: file
  path: targets.yaml
```

```shell
gnmic --config gnmic.yaml proxy
```

#### proxy with service registration

```yaml
gnmi-server:
  address: gnmi-proxy-address:57401
  service-registration:
    name: proxy
    address: consul-server:8500

loader:
  type: file
  path: targets.yaml
```

```shell
gnmic --config gnmic.yaml proxy
```
