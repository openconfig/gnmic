# Collector Configuration

This page describes the configuration options specific to the collector mode. For general configuration options (targets, subscriptions, outputs, inputs, processors), refer to their respective documentation pages.

## API Server

The API server is required for the collector to accept configuration changes and serve status information.

```yaml
api-server:
  # string, address to listen on in the form "host:port"
  # the host part can be omitted to listen on all interfaces
  address: :7890
  # duration, request timeout
  # split equally between read and write timeouts
  timeout: 10s 
  # TLS configuration for secure API access
  tls:
    # string, path to CA certificate file
    # used to verify client certificates
    ca-file:
    # string, path to server certificate file
    cert-file:    
    # string, path to server private key file
    key-file:
    # string, client authentication mode
    # one of: "", "request", "require", "verify-if-given", "require-verify"
    #
    # - "":              no client certificate requested
    # - "request":       request certificate, don't require it, don't verify
    # - "require":       require certificate, don't verify
    # - "verify-if-given": request certificate, verify if provided
    # - "require-verify": require and verify certificate
    #
    # defaults to "" if no ca-file, "require-verify" if ca-file is set
    client-auth: ""
  
  # boolean, enable Prometheus metrics endpoint at /metrics
  enable-metrics: false
  # boolean, enable debug logging for API requests
  debug: false
```

## Clustering

Clustering enables multiple collector instances to work together for high availability and load distribution.

```yaml
clustering:
  # string, cluster name
  # instances with the same cluster name form a cluster
  # used in leader lock key and target lock keys
  # defaults to "default-cluster"
  cluster-name: default-cluster
  # string, unique instance name within the cluster
  # used as value in target locks and leader lock
  # defaults to "gnmic-$UUID" if not set
  instance-name: ""  
  # string, service address to register with the locker (e.g., Consul)
  # defaults to the address part of api-server address
  service-address: ""
  # duration, how long to watch for service changes (Consul blocking query)
  # defaults to 60s
  services-watch-timer: 60s
  # duration, interval between target distribution checks by the leader
  # defaults to 20s
  targets-watch-timer: 20s
  # duration, max time to wait for an instance to lock an assigned target
  # if exceeded, leader reassigns the target to another instance
  # defaults to 10s
  target-assignment-timeout: 10s
  # duration, time to wait after becoming leader before distributing targets
  # allows other instances to register their API services
  # defaults to 5s
  leader-wait-timer: 5s
  # tags used for target placement decisions
  # targets with matching tags are preferentially assigned to this instance
  tags: []
  # locker configuration (required for clustering)
  locker:
    # string, locker type
    type: consul 
    # string, locker server address
    address: localhost:8500
    # string, datacenter name (Consul-specific)
    datacenter: dc1
    # string, username for HTTP basic auth
    username:
    # string, password for HTTP basic auth  
    password:    
    # string, ACL token
    token:
    # duration, session TTL
    session-ttl: 10s
    # duration, delay before lock can be acquired after release
    delay: 5s
    # duration, time between lock retry attempts
    retry-timer: 2s
    # boolean, enable debug logging
    debug: false
```

<!-- ## gNMI Server

The embedded gNMI server allows the collector to serve collected data to downstream gNMI clients.

```yaml
gnmi-server:
  # string, address to listen on
  address: :57400
  
  # TLS configuration
  tls:
    ca-file:
    cert-file:
    key-file:
    client-auth: ""
  
  # integer, maximum concurrent subscribe RPCs
  max-subscriptions: 64
  
  # integer, maximum concurrent Get/Set RPCs
  max-unary-rpc: 64
  
  # integer, maximum receive message size in bytes
  # defaults to 4MB
  max-recv-msg-size:
  
  # integer, maximum send message size in bytes
  # defaults to MaxInt32
  max-send-msg-size:
  
  # integer, maximum concurrent streams per RPC
  max-concurrent-streams:
  
  # duration, TCP keepalive time and interval
  # negative value disables keepalive
  tcp-keepalive:
  
  # gRPC keepalive configuration
  keepalive:
    max-connection-idle:
    max-connection-age:
    max-connection-age-grace:
    time: 120m
    timeout: 20s
  
  # duration, minimum sample interval
  # enforced when client requests smaller interval
  min-sample-interval: 1ms
  
  # duration, default sample interval
  # used when client requests 0 interval
  default-sample-interval: 1s
  
  # duration, minimum heartbeat interval
  min-heartbeat-interval: 1s
  
  # boolean, enable Prometheus gRPC metrics
  enable-metrics: false
  
  # boolean, enable debug logging
  debug: false
  
  # cache configuration for the gNMI server
  cache:
    # string, cache type: "oc" (OpenConfig) or "redis"
    type: oc
    
    # string, address (for redis type)
    address:
    
    # string, username (for redis type)
    username:
    
    # string, password (for redis type)
    password:
    
    # duration, cache expiration time
    expiration: 0s
    
    # boolean, enable debug logging
    debug: false
  
  # Consul service registration
  service-registration:
    address:
    datacenter:
    username:
    password:
    token:
    check-interval: 5s
    max-fail: 3
    name:
    tags: []
``` -->

## Tunnel Server

The tunnel server accepts connections from gNMI tunnel targets.

```yaml
tunnel-server:
  # string, address to listen on
  address: :57401
  
  # TLS configuration
  tls:
    ca-file:
    cert-file:
    key-file:
    client-auth: ""
  
  # boolean, enable debug logging
  debug: false
```

## Tunnel Target Matches

Define rules for handling tunnel target connections.

```yaml
tunnel-target-matches:
  # match rule name
  match-all:
    # string, target id to match (from tunnel target Register RPC)
    id: "*"
    # string, target type to match, typically GNOI_GNMI (from tunnel target Register RPC)
    type: "GNMMI_GNOI"
    
    # list of subscription names to apply
    subscriptions:
      - interfaces
      - system
    
    # list of output names to send data to
    outputs:
      - prometheus
```

Note that tunnel-target-matches are not processed in any specific order. It's adviced to make sure there is no overlap between the rules `type` and `id`.

## Complete Example

```yaml
# API server (required)
api-server:
  address: :7890
  timeout: 10s
  enable-metrics: true

# Clustering (optional, for HA)
clustering:
  cluster-name: production-cluster
  instance-name: collector-1
  locker:
    type: consul
    address: consul.service.consul:8500
    session-ttl: 10s

# gNMI server
gnmi-server:
  address: :57400
  skip-verify: true
  cache:
    type: oc
    expiration: 60s

# Tunnel server
tunnel-server:
  address: :57401

# Tunnel target matches
tunnel-target-matches:
  srl-devices:
    id: router1
    type: "GNMI_GNOI"
    subscriptions:
      - interfaces
    outputs:
      - prometheus

# Targets
targets:
  spine1:
    address: 10.0.0.1:57400
    username: admin
    password: admin
    skip-verify: true
    subscriptions:
      - interfaces
      - bgp
    outputs:
      - prometheus

# Subscriptions
subscriptions:
  interfaces:
    paths:
      - /interfaces/interface/state/counters
    mode: stream
    stream-mode: sample  
    sample-interval: 10s
  
  bgp:
    paths:
      - /network-instances/network-instance/protocols/protocol/bgp
    mode: stream
    stream-mode: on-change

# Outputs
outputs:
  prometheus:
    type: prometheus
    listen: :9804
    path: /metrics
    event-processors:
      - trim-prefixes

# Processors
processors:
  trim-prefixes:
    event-strings:
      value-names:
        - ".*"
      transforms:
        - trim-prefix:
            apply-on: name
            prefix: /interfaces/interface/state/
```
