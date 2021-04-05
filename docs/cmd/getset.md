### Description

The `getset` command is a combination of the gNMI [Get RPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L57) and the gNMI [Set RPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L62).

It allows to conditionally execute a [Set RPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L62) based on a condition evaluated against a [GetResponse](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L420).

The `condition` written as a [`jq expression`](https://stedolan.github.io/jq/), is specified using the flag `--condition`.

The [SetRPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L62) is executed only if the condition evaluates to `true`

### Usage

`gnmic [global-flags] getset [local-flags]`

`gnmic [global-flags] gas [local-flags]`

`gnmic [global-flags] gs [local-flags]`


### Flags

#### prefix

As per [path prefixes](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#241-path-prefixes), the prefix `[--prefix]` flag represents a common prefix that is applied to all paths specified using the local `--get`, `--update`, `--replace` and `--delete` flags. 

Defaults to `""`.

#### get
The mandatory get flag `[--get]` is used to specify the single [path](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#222-paths) used in the Get RPC.

#### model

The optional model flag `[--model]` is used to specify the schema definition modules that the target should use when returning a GetResponse. The model name should match the names returned in Capabilities RPC. Currently only single model name is supported.

#### target
With the optional `[--target]` flag it is possible to supply the [path target](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#2221-path-target) information in the prefix field of the GetRequest message.

#### type

The type flag `[--type]` is used to specify the [data type](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L399) requested from the server.

One of:  ALL, CONFIG, STATE, OPERATIONAL (defaults to "ALL")

#### condition
The `[--condition]` is a [`jq expression`](https://stedolan.github.io/jq/) that can be used to determine if the Set Request is executed based on the Get Response values.

#### update
The `[--update]` specifies a [`jq expression`](https://stedolan.github.io/jq/) used to build the Set Request update path.

#### replace
The `[--replace]` specifies a [`jq expression`](https://stedolan.github.io/jq/) used to build the Set Request replace path.

#### delete
The `[--delete]` specifies a [`jq expression`](https://stedolan.github.io/jq/) used to build the Set Request delete path.

#### value
The `[--value]` specifies a [`jq expression`](https://stedolan.github.io/jq/) used to build the Set Request value.

### Examples

The command in the below example does the following:

- gets the list of interface indexes to interface name mapping, 

- checks if the interface index (ifindex) 70 exists,

- if it does, the set request changes the interface state to `enable` using the interface name.

```bash
gnmic getset -a <ip:port> \
    --get /interface/ifindex \
    --condition '.[] | .updates[].values[""]["srl_nokia-interfaces:interface"][] | select(.ifindex==70) | (.name != "" or .name !=null)' \
    --update '.[] | .updates[].values[""]["srl_nokia-interfaces:interface"][] | select(.ifindex==70) | "interface[name=" + .name + "]/admin-state"' \
    --value enable
```