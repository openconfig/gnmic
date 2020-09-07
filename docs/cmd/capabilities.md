## Description
The `[cap | capabilities]` command represents the [gNMI Capabilities RPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L51).

It is used to send a [Capability Request](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L431) to the specified target(s) and expects one [Capability Response](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L440) per target.

[Capabilities](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#32-capability-discovery) allows the client to retrieve the set of capabilities that is supported by the target:

* gNMI version
* available data models
* supported encodings
* gNMI extensions

This allows the client to, for example, validate the service version that is implemented and retrieve the set of models that the target supports. The models can then be specified in subsequent Get/Subscribe RPCs to precisely tell the target which models to use.

### Usage

`gnmic [global-flags] capabilities [local-flags]`

### Examples

#### single host

```text
gnmic -a <ip:port> --username <user> --password <password> \
      --insecure capabilities

gNMI_Version: 0.7.0
supported models:
  - nokia-conf, Nokia, 19.10.R2
  - nokia-state, Nokia, 19.10.R2
  - nokia-li-state, Nokia, 19.10.R2
  - nokia-li-conf, Nokia, 19.10.R2
<< SNIPPED >>
supported encodings:
  - JSON
  - BYTES
```

#### multiple hosts

```bash
gnmic -a <ip:port>,<ip:port> -u <user> -p <password> \
      --insecure cap
```



<script id="asciicast-319561" src="https://asciinema.org/a/319561.js" async></script>