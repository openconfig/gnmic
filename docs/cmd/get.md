### Description

The `get` command represents the gNMI [Get RPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L57).

It is used to send a [GetRequest](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L395) to the specified target(s) (using the global flag [`--address`](../global_flags.md#address) and expects one [GetResponse](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L420) per target, per path.

The [Get RPC](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#33-retrieving-snapshots-of-state-information) is used to retrieve a snapshot of data from the target. It requests that the target snapshots a subset of the data tree as specified by the paths included in the message and serializes this to be returned to the client using the specified encoding.

### Usage

`gnmic [global-flags] get [local-flags]`

### Flags

#### prefix

As per [path prefixes](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#241-path-prefixes), the prefix `[--prefix]` flag represents a common prefix that is applied to all paths specified using the local `--path` flag. Defaults to `""`.

#### path

The mandatory path flag `[--path]` is used to specify the [path(s)](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#222-paths) the client wants to receive a snapshot of.

Multiple paths can be specified by using multiple `--path` flags:

```bash
gnmic -a <ip:port> --insecure \
      get --path "/state/ports[port-id=*]" \
          --path "/state/router[router-name=*]/interface[interface-name=*]"
```

If a user needs to provide [origin](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#222-paths) information to the Path message, the following pattern should be used for the path string: `"origin:path"`:

!!! note
    The path after the origin value has to start with a `/`

```
gnmic -a <ip:port> --insecure \
      get --path "openconfig-interfaces:/interfaces/interface"
```

#### model

The optional model flag `[--model]` is used to specify the schema definition modules that the target should use when returning a GetResponse. The model name should match the names returned in Capabilities RPC. Currently only single model name is supported.

#### target

With the optional `[--target]` flag it is possible to supply the [path target](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#2221-path-target) information in the prefix field of the GetRequest message.

#### values-only

The flag `[--values-only]` allows to print only the values returned in a GetResponse. This is useful when only the value of a leaf is of interest, like check if a value was set correctly.

#### type

The type flag `[--type]` is used to specify the [data type](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L399) requested from the server.

One of:  ALL, CONFIG, STATE, OPERATIONAL (defaults to "ALL")

#### processor

The `[--processor]` flag allow to list [event processor](../user_guide/event_processors/intro.md) names to be run as a result of receiving the GetReponse messages.

The processors are run in the order they are specified (`--processor proc1,proc2` or `--processor proc1 --processor proc2`).

### Examples

```bash
# simple Get RPC
gnmic -a <ip:port> get --path "/state/port[port-id=*]"

# Get RPC with multiple paths
gnmic -a <ip:port> get --path "/state/port[port-id=*]" \
      --path "/state/router[router-name=*]/interface[interface-name=*]"

# Get RPC with path prefix
gnmic -a <ip:port> get --prefix "/state" \
      --path "port[port-id=*]" \
      --path "router[router-name=*]/interface[interface-name=*]"
```

<script
id="asciicast-319562" src="https://asciinema.org/a/319562.js" async>
</script>
