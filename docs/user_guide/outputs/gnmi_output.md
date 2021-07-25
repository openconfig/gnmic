`gnmic` supports acting as a `gNMI Server` to expose the subscribed telemetry data to a `gNMI Client` using the `Subcribe` RPC, or to act as a gateway for `Get` and `Set` RPCs.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/gnmi_server.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fgnmi_server.drawio" async></script>

### Configuration

```yaml
outputs:
  output1:
    # required
    type: gnmi 
    # gNMI server address, either a TCP socket or UNIX socket. 
    # In the latter case, the prefix `unix:///` should be present.
    address: ":57400"
    # maximum number of active subscriptions.
    max-subscriptions: 64
    # maximum number of ongoing Get/Set RPCs.
    max-unary-rpc: 64
    # boolean, if true, the gNMI server will run in secure mode 
    # but will not verify the client certificate against the available certificate chain.
    skip-verify: false
    # string, path to the CA certificate file, this will be used to verify the clients certificates, if `skip-verify` is false
    ca-file:
    # string, server certificate file.
    # if both `cert-file` and `key-file` are empty, and `skip-verify` is true or `ca-file` is set, 
    # the server will run with self signed certificates.
    cert-file:
    # string, server key file.
    # if both `cert-file` and `key-file` are empty, and `skip-verify` is true or `ca-file` is set, 
    # the server will run with self signed certificates.
    key-file:
    # string, a GoTemplate that allow for the customization of the target field in Prefix.Target.
    # it applies only if the returned Prefix.Target is empty.
    # if left empty, it defaults to:
    # `{{- if index . "subscription-target" -}}
    # {{ index . "subscription-target" }}
    # {{- else -}}
    # {{ index . "source" | host }}
    # {{- end -}}`
    # which will set the target to the value configured under `subscription.$subscription-name.target` if any,
    # otherwise it will set it to the target name stripped of the port number (if present).
    target-template:
    # boolean, enables extra logging for the gNMI Server
    debug: false
    # boolean, enables the collection and export (via prometheus) of output specific metrics
    enable-metrics: false 
```

#### Insecure Mode

By default, the server runs in insecure mode, as long as `skip-verify` is false and none of `ca-file`, `cert-file` and `key-file` are set.

#### Secure Mode

To run this gNMI server in secure mode, there are a few options:

- **Using self signed certificates, without client certificate verification:**

```yaml
skip-verify: true
```

- **Using self signed certificates, with client certificate verification:**

```yaml
# a valid CA certificate to verify the client provided certificates
ca-file: /path/to/caFile 
```
  
- **Using CA provided certificates, without client certificate verification:**

```yaml
skip-verify: true
# a valid server certificate
cert-file: /path/to/server-cert
# a valid server key
key-file:  /path/to/server-key
```

- **Using CA provided certificates, with client certificate verification:**

```yaml
# a valid CA certificate to verify the client provided certificates
ca-file: /path/to/caFile 
# a valid server certificate
cert-file: /path/to/server-cert
# a valid server key
key-file:  /path/to/server-key
```

### Supported RPCs

This `gNMI Server` supports `Get`, `Set` and `Subscribe` RPCs.

#### gNMI Subscribe RPC

The server keeps a cache of gNMI notifications synched with the configured targets based on the configured subscriptions.
This means that a client cannot get updates about a leaf that `gNMIc` did not subscribe to upstream.

As soon as there is an update to the cache, the added gNMI notification is sent to all the client which subscription matches the new notification.

Clients can subscribe to specific target using the gNMI Prefix Target field, leaving the Target field empty or setting it to `*` is equivalent to subscribing to all known targets.

#### gNMI Get RPC

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/gnmi_server.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fgnmi_server.drawio" async></script>

The server supports the gNMI `Get` RPC.
It relies on the Prefix.Target field to select the target(s) to relay the received GetRequest to.

If Prefix.Target is empty or is equal to `*`, a Get RPC is performed for all known targets.
The received GetRequest is cloned, enriched with each target name and sent to the corresponding destination.

Comma separated target names are also supported and allow to select a list of specific targets to send the Get RPC to.

Once all GetResponses are received back successfully, the notifications contained in each GetResponse are combined into a single GetResponse with their Prefix.Target populated, if empty.

The resulting GetResponse is then returned to the gNMI client.
If one of the RPCs fails, an error with status code `Internal(13)` is returned to the client.

If the Get Request has the origin field set to `gnmic`, the request is performed against the internal server configuration.
Currently only the path `targets` is supported.

```bash
gnmic -a localhost:57400 --skip-verify get --path gnmic:/targets
```

```json
[
  {
    "timestamp": 1626759382486891218,
    "time": "2021-07-20T13:36:22.486891218+08:00",
    "prefix": "gnmic:targets[name=clab-gw-srl1:57400]",
    "updates": [
      {
        "Path": "address",
        "values": {
          "address": "clab-gw-srl1:57400"
        }
      },
      {
        "Path": "username",
        "values": {
          "username": "admin"
        }
      },
      {
        "Path": "insecure",
        "values": {
          "insecure": "false"
        }
      },
      {
        "Path": "skip-verify",
        "values": {
          "skip-verify": "true"
        }
      },
      {
        "Path": "timeout",
        "values": {
          "timeout": "10s"
        }
      }
    ]
  },
  {
    "timestamp": 1626759382486900697,
    "time": "2021-07-20T13:36:22.486900697+08:00",
    "prefix": "gnmic:targets[name=clab-gw-srl2:57400]",
    "updates": [
      {
        "Path": "address",
        "values": {
          "address": "clab-gw-srl2:57400"
        }
      },
      {
        "Path": "username",
        "values": {
          "username": "admin"
        }
      },
      {
        "Path": "insecure",
        "values": {
          "insecure": "false"
        }
      },
      {
        "Path": "skip-verify",
        "values": {
          "skip-verify": "true"
        }
      },
      {
        "Path": "timeout",
        "values": {
          "timeout": "10s"
        }
      }
    ]
  }
]
```

#### gNMI Set RPC

The gNMI server supports the gNMI `Set` RPC.
Just like in the case of `Get` RPC, the server relies on the `Prefix.Target` field to select the target(s) to relay the received SetRequest to.

If Prefix.Target is empty or is equal to `*`, a Set RPC is performed for all known targets.
The received SetRequest is cloned, enriched with each target name and sent to the corresponding destination.

Comma separated target names are also supported and allow to select a list of specific targets to send the Set RPC to.

Once all SetResponses are received back successfully, the `UpdateResult`s from each response are merged into a single SetResponse, with the addition of the target name set in `Path.Target`.
This is not compliant with the gNMI specification which stipulates that the `Target` field should only be present in [Prefix Paths](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#2221-path-target)

The resulting SetResponse is then returned to the gNMI client.
If one of the RPCs fails, an error with status code `Internal(13)` is returned to the client.
