# gNMI Server

## Introduction

On top of acting as `gNMI` client `gNMIc` can run a `gNMI` server that supports `Get`, `Set` and `Subscribe` RPCs.

The goal is to act as a caching point for the collected gNMI notifications and make them available to other collectors via the `Subscribe` RPC.

Using this gNMI server feature it is possible to build `gNMI` based clusters and pipelines with `gNMIc`.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/gnmi_server.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fgnmi_server.drawio" async></script>

The server keeps a cache of the gNMI notifications received from the defined subscriptions and utilizes it to build the `Subscribe` RPC responses.

The unary RPCs, Get and Set, are relayed to known targets based on the `Prefix.Target` field.

## Supported features

- Supports gNMI RPCs, Get, Set, Subscribe
- Acts as a gNMI gateway for Get and Set RPCs.
- Supports Service registration with Consul server.
- Supports all types of gNMI subscriptions, `once`, `poll`, `stream`.
- Supports all types of `stream` subscriptions, `on-change`, `target-defined` and `sample`.
- Supports `updates-only` with `stream` and `once` subscriptions.
- Supports `suppress-redundant`.
- Supports `heartbeat-interval` with `on-change` and `sample` stream subscriptions.

## Get RPC

The server supports the gNMI `Get` RPC, it allows a client to retrieve `gNMI` notifications from multiple targets into a single `GetResponse`.

It relies on the `GetRequest` `Prefix.Target` field to select the target(s) against which it will run the Get RPC.

If `Prefix.Target` is left empty or is equal to `*`, the Get RPC is performed against all known targets.
The received GetRequest is cloned, enriched with each target name and sent to the corresponding destination.

Comma separated target names are also supported and allow to select a list of specific targets to send the Get RPC to.

```bash
gnmic -a gnmic-server:57400 get --path /interfaces \
                                --target router1,router2,router3
```

Once all GetResponses are received back successfully, the notifications contained in each GetResponse are combined into a single GetResponse with each notification's `Prefix.Target` populated, if empty.

The resulting GetResponse is then returned to the gNMI client.
If one of the RPCs fails, an error with status code `Internal(13)` is returned to the client.

If the GetRequest Path has the `Origin` field set to `gnmic`, the request is performed against the internal `gNMIc` server configuration.
Currently only the paths `targets` and `subscriptions` are supported.

```bash
gnmic -a gnmic-server:57400 get --path gnmic:/targets
gnmic -a gnmic-server:57400 get --path gnmic:/subscriptions
```

## Set RPC

This `gNMI` server supports the gNMI `Set` RPC, it allows a client to run a single `Set` RPC against multiple targets.

Just like in the case of `Get` RPC, the server relies on the `Prefix.Target` field to select the target(s) against which it will run the `Set` RPC.

If `Prefix.Target` is left empty or is equal to `*`, a Set RPC is performed against all known targets.
The received SetRequest is cloned, enriched with each target name and sent to the corresponding destination.

Comma separated target names are also supported and allow to select a list of specific targets to send the Set RPC to.

```bash
gnmic -a gnmic-server:57400 set \
        --update /system/ssh-server/admin-state:::json:::disable \
        --target router1,router2,router3
```

Once all SetResponses are received back successfully, the `UpdateResult`s from each response are merged into a single SetResponse, with the addition of the target name set in `Path.Target`.

!!! note
    Adding a target value to a non prefix path is not compliant with the gNMI specification which stipulates that the `Target` field should only be present in [Prefix Paths](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#2221-path-target)

The resulting SetResponse is then returned to the gNMI client.
If one of the RPCs fails, an error with status code `Internal(13)` is returned to the client.

## Subscribe RPC

The `gNMIc` server keeps a cache of gNMI notifications synched with the configured targets based on the configured subscriptions.

The Subscribe requests received from a client are run against the afore mentioned cache,
this means that a client cannot get updates about a leaf that `gNMIc` did not subscribe to as a client.

Clients can subscribe to specific target using the gNMI `Prefix.Target` field,
while leaving the `Prefix.Target` field empty or setting it to `*` is equivalent to subscribing to all known targets.

### Subscription Mode

`gNMIc` gNMI Server supports the 3 gNMI specified subscription modes: `Once`, `Poll` and `Stream`.

It also supports some subscription behavior modifiers:

- `updates-only` with `stream` and `once` subscriptions.
- `suppress-redundant`.
- `heartbeat-interval` with `on-change` and `sample` stream subscriptions.

#### [Once](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#35151-once-subscriptions)

A subscription operating in the `ONCE` mode acts as a single request/response channel.
The target creates the relevant update messages, transmits them, and subsequently closes the RPC.

In this subscription mode, `gNMIc` server supports the `updates-only` knob.

#### [Poll](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#35153-poll-subscriptions)

Polling subscriptions are used for on-demand retrieval of data items via long-lived RPCs. A poll subscription relates to a certain set of subscribed paths, and is initiated by sending a SubscribeRequest message with encapsulated SubscriptionList. Subscription messages contained within the SubscriptionList indicate the set of paths that are of interest to the polling client.

#### [Stream](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#35152-stream-subscriptions)

Stream subscriptions are long-lived subscriptions which continue to transmit updates relating to the set of paths that are covered within the subscription indefinitely.

In this subscription mode, `gNMIc` server supports the `updates-only` knob.

##### On Change

When a subscription is defined to be `on-change`, data updates are only sent to the client when the value of the data item changes.

In the case of `gNMIc` gNMI server, `on-change` subscriptions depend on the subscription writing data in the local cache,
if it is a `sample` subscription, each update from a target will trigger an `on-change` update to the server client.

`gNMIc` gNMI server supports `on-change` subscriptions with `heartbeat-interval`.
If the `heartbeat-interval` value is set to a non zero value, the value of the data item(s) MUST be re-sent once per heartbeat interval regardless of whether the value has changed or not.

!!! note
    The minimum heartbeat-interval is configurable using the field `min-heartbeat-interval`. It defaults to `1s`

    If the received `heartbeat-interval` value is greater than zero but lower than `min-heartbeat-interval`, the `min-heartbeat-interval` value is used instead.

##### Target Defined

When a client creates a subscription specifying the target defined mode, the target MUST determine the best type of subscription to be created on a per-leaf basis.

In the case of `gNMIc` gNMI server, a `target-defined` stream subscription, is treated as an `on-change` subscription.

Note that this does not mean that `gNMIc` will filter out the unchanged values received from a sample subscription to the actual targets.

##### Sample

A `sample` subscription is one where data items are sent to the client once per `sample-interval`.

The minimum supported `sample-interval` is configurable using the field `min-sample-interval`, defaults to `1ms`.

If within a `SubscribeRequest` the received `sample-interval` is zero, the `default-sample-interval` is used, defaults to `1s`.

## Configuration

```yaml
gnmi-server:
  # the address the gNMI server will listen to
  address: :57400
  # if true, the server will not verify the client's certificates
  skip-verify: false
  # path to the CA certificate file to be used, irrelevant if `skip-verify` is true
  ca-file: 
  # path to the server certificate file
  cert-file:
  # path to the server key file
  key-file:
  # maximum number of allowed subscriptions
  max-subscriptions: 64
  # maximum number of active Get/Set RPCs
  max-unary-rpc: 64
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

### Secure vs Insecure Server

#### Insecure Mode

By default, the server runs in insecure mode, as long as `skip-verify` is false and none of `ca-file`, `cert-file` and `key-file` are set.

#### Secure Mode

To run this gNMI server in secure mode, there are a few options:

- **Using self signed certificates, without client certificate verification:**

```yaml
gnmi-server:
  skip-verify: true
```

- **Using self signed certificates, with client certificate verification:**

```yaml
gnmi-server:
# a valid CA certificate to verify the client provided certificates
  ca-file: /path/to/caFile 
```
  
- **Using CA provided certificates, without client certificate verification:**

```yaml
gnmi-server:
  skip-verify: true
  # a valid server certificate
  cert-file: /path/to/server-cert
  # a valid server key
  key-file:  /path/to/server-key
```

- **Using CA provided certificates, with client certificate verification:**

```yaml
gnmi-server:
  # a valid CA certificate to verify the client provided certificates
  ca-file: /path/to/caFile 
  # a valid server certificate
  cert-file: /path/to/server-cert
  # a valid server key
  key-file:  /path/to/server-key
```

### Fields

#### address

Defines the address the gNMI server will listen to.

This can be a tcp socket in the format `<addr:port>` or a unix socket starting with `unix:///`

#### skip-verify

If true, the server will not verify the client's certificates.

#### ca-file

Defines the path to the CA certificate file to be used, irrelevant if `skip-verify` is true

#### cert-file

Defines the path to the server certificate file to be used.

#### key-file

Defines the path to the server key file to be used.

#### max-subscriptions

Defines the maximum number of allowed subscriptions.

Defaults to `64`.

#### max-unary-rpc

Defines the maximum number of active Get/Set RPCs.

Defaults to `64`.

#### min-sample-interval

Defines the minimum allowed sample interval, this value is used when the received sample-interval
is greater than zero but lower than this minimum value.

Defaults to `1ms`
  
#### default-sample-interval

Defines the default sample interval,
this value is used when the received sample-interval is zero within a stream/sample subscription.

Defaults to `1s`

#### min-heartbeat-interval

Defines the minimum heartbeat-interval,
this value is used when the received heartbeat-interval is greater than zero but
lower than this minimum value.

Defaults to `1s`

#### enable-metrics

Enables the collection of Prometheus gRPC server metrics.

#### debug

Enables additional debug logging.
