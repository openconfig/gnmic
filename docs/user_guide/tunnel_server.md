# Tunnel Server

## Introduction

`gNMIc` supports gNMI Dial-out as defined by [`openconfig/grpctunnel`](https://github.com/openconfig/grpctunnel).

`gNMIc` embeds a tunnel server to which the gNMI targets register. Once registered, `gNMIc` triggers the request gNMI RPC towards the target via the established tunnel.

This use case is described [here](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmignoissh-dialout-grpctunnel.md#gnmi-collector-with-embedded-tunnel-server)

## Server operation

When running a Subscribe RPC using `gNMIc` with the flag `--use-tunnel-server`,`gNMIc` starts by running the Tunnel server as defined under `tunnel-server`.

The next steps depend on the type of RPC (Unary/Stream) and/or Subscribe Mode (poll/once/stream)

### Unary RPCs

`gNMIc` waits for `tunnel-server.target-wait-time` for targets to register with the tunnel server, after which it requests a new session from the server for the specified target(s) and runs the RPC through the newly established tunnel.

Note that if no target is specified, the RPC runs for all registered targets.

```bash
$ cat tunnel_server_config.yaml
insecure: true
log: true
username: admin
password: admin

tunnel-server:
  address: ":57401"
```

```bash
$ gnmic --config tunnel_server_config.yaml \
      --use-tunnel-server \
      get \
      --path /configure/system/name
2022/03/09 10:12:34.729037 [gnmic] version=dev, commit=none, date=unknown, gitURL=, docs=https://gnmic.kmrd.dev
2022/03/09 10:12:34.729063 [gnmic] using config file "tunnel_server_config.yaml"
2022/03/09 10:12:34.730472 [gnmic] waiting for targets to register with the tunnel server...
2022/03/09 10:12:36.435521 [gnmic] tunnel server discovered target {ID:sr1 Type:GNMI_GNOI}
2022/03/09 10:12:36.436332 [gnmic] tunnel server discovered target {ID:sr2 Type:GNMI_GNOI}
2022/03/09 10:12:36.731125 [gnmic] adding target {"name":"sr1","address":"sr1","username":"admin","password":"admin","timeout":10000000000,"insecure":true,"skip-verify":false,"subscriptions":["sub1"],"retry-timer":10000000000,"log-tls-secret":false,"gzip":false,"token":""}
2022/03/09 10:12:36.731158 [gnmic] adding target {"name":"sr2","address":"sr2","username":"admin","password":"admin","timeout":10000000000,"insecure":true,"skip-verify":false,"subscriptions":["sub1"],"retry-timer":10000000000,"log-tls-secret":false,"gzip":false,"token":""}
2022/03/09 10:12:36.731651 [gnmic] sending gNMI GetRequest: prefix='<nil>', path='[elem:{name:"configure"}  elem:{name:"system"}  elem:{name:"name"}]', type='ALL', encoding='JSON', models='[]', extension='[]' to sr1
2022/03/09 10:12:36.731742 [gnmic] sending gNMI GetRequest: prefix='<nil>', path='[elem:{name:"configure"}  elem:{name:"system"}  elem:{name:"name"}]', type='ALL', encoding='JSON', models='[]', extension='[]' to sr2
2022/03/09 10:12:36.732337 [gnmic] dialing tunnel connection for tunnel target "sr2"
2022/03/09 10:12:36.732572 [gnmic] dialing tunnel connection for tunnel target "sr1"
[sr1] [
[sr1]   {
[sr1]     "source": "sr1",
[sr1]     "timestamp": 1646849561604621769,
[sr1]     "time": "2022-03-09T10:12:41.604621769-08:00",
[sr1]     "updates": [
[sr1]       {
[sr1]         "Path": "configure/system/name",
[sr1]         "values": {
[sr1]           "configure/system/name": "sr1"
[sr1]         }
[sr1]       }
[sr1]     ]
[sr1]   }
[sr1] ]
[sr2] [
[sr2]   {
[sr2]     "source": "sr2",
[sr2]     "timestamp": 1646849562004804732,
[sr2]     "time": "2022-03-09T10:12:42.004804732-08:00",
[sr2]     "updates": [
[sr2]       {
[sr2]         "Path": "configure/system/name",
[sr2]         "values": {
[sr2]           "configure/system/name": "sr2"
[sr2]         }
[sr2]       }
[sr2]     ]
[sr2]   }
[sr2] ]
```

### Subscribe RPC

#### Poll and Once subscription

When a Poll or Once subscription are requested, `gNMIc` behaves the same way as for a unary RPC, i.e waits for targets to register then runs the RPC.

#### Stream subscription

In the case of a stream subscription, `gNMIc` triggers the Subscribe RPC as soon as a target registers.
Similarly, a stream subscription will be stopped when a target deregisters from the tunnel server.

## Configuration

```yaml
tunnel-server:
  # the address the tunnel server will listen to
  address:
  # if true, the server will not verify the client's certificates
  skip-verify: false
  # path to the CA certificate file to be used, irrelevant if `skip-verify` is true
  ca-file: 
  # path to the server certificate file
  cert-file:
  # path to the server key file
  key-file:
  # the wait time before triggering unary RPCs or subscribe poll/once
  target-wait-time: 2s
  # enables the collection of Prometheus gRPC server metrics
  enable-metrics: false
  # enable additional debug logs
  debug: false
```

## Combining Tunnel server with a gNMI server

It is possible to start `gNMIc` with both a `gnmi-server` and `tunnel-server` enabled.

This mode allows to run gNMI RPCs against `gNMIc`'s gNMI server, they will routed to the relevant targets (`--target` flag) or to all known target (i.e registered targets)

The configuration file would look like:

```yaml
insecure: true
username: admin
password: admin

subscriptions:
  sub1:
    paths:
      - /state/port
    sample-interface: 10s

gnmi-server:
  address: :57400

tunnel-server:
  address: :57401
  targets:
    - id: .*
      type: GNMI_GNOI
      config:
        subscriptions:
          - sub1
```

Running a Get RPC towards all registered targets

```bash
$ gnmic -a localhost:57400 --insecure get \
        --path /configure/system/name
[
  {
    "source": "localhost",
    "timestamp": 1646850987401608313,
    "time": "2022-03-09T10:36:27.401608313-08:00",
    "target": "sr2",
    "updates": [
      {
        "Path": "configure/system/name",
        "values": {
          "configure/system/name": "sr2"
        }
      }
    ]
  },
  {
    "source": "localhost",
    "timestamp": 1646850987205206394,
    "time": "2022-03-09T10:36:27.205206394-08:00",
    "target": "sr1",
    "updates": [
      {
        "Path": "configure/system/name",
        "values": {
          "configure/system/name": "sr1"
        }
      }
    ]
  }
]
```

Running a Get RPC towards a single target

```bash
$ gnmic -a localhost:57400 --insecure \
        --target sr1 \
        get --path /configure/system/name
[
  {
    "source": "localhost",
    "timestamp": 1646851044004381267,
    "time": "2022-03-09T10:37:24.004381267-08:00",
    "target": "sr1",
    "updates": [
      {
        "Path": "configure/system/name",
        "values": {
          "configure/system/name": "sr1"
        }
      }
    ]
  }
]
```

For detailed configuration of the `gnmi-server` check this [page](./gnmi_server.md)
