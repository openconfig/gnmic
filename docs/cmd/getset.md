### Description

The `getset` command is a combination of the gNMI [Get RPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L57) and the gNMI [Set RPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L62).

It is used to send a [GetRequest](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L395) to the specified target(s) (using the global flag [`--address`](../global_flags.md#address) and expects one [GetResponse](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L420) per target.

The [GetResponse](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L420)) is then used to evaluate a `condition` written as a [`jq expression`](https://stedolan.github.io/jq/), specified with the flag `--condition`.

If the condition evaluates to `true` then the [SetRPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L62) is executed.

### Usage

`gnmic [global-flags] getset [local-flags]`

`gnmic [global-flags] gas [local-flags]`

`gnmic [global-flags] gs [local-flags]`


### Flags

#### prefix

As per [path prefixes](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#241-path-prefixes), the prefix `[--prefix]` flag represents a common prefix that is applied to all paths specified using the local `--get`, `--update`, `--replace` and `--delete` flags. Defaults to `""`.

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
The `[--condition]` is a `jq` expression that can be used to control if the Set Request is executed based on the Get Response values.

#### update
The `[--update]` specifies a Go template or a jq expression used to build the Set Request update path.

#### replace
The `[--replace]` specifies a Go template or a jq expression used to build the Set Request replace path.

#### delete
The `[--delete]` specifies a Go template or a jq expression used to build the Set Request delete path.

#### value
The `[--value]` specifies a Go template or a jq expression used to build the Set Request value.

### Examples

```bash
# get the list of interface indexes to interface name mapping, 
# check if ifindex 70 exists,
# if it does, change the state to `enable` using the interface name

# using Go template
gnmic getset -a <ip:port> \
    --get /interface/ifindex \
    --condition '.[] | .updates[].values[""]["srl_nokia-interfaces:interface"][] | select(.ifindex==70) | (.name != "" or .name !=null)' \
    --update 'interface[name={{range (select . "" "srl_nokia-interfaces:interface" )}}{{range .}}{{ if eq  (int (index . "ifindex")) 70}}{{ index . "name" }}{{end}}{{end}}{{end}}]/admin-state' \
    --value enable

# using jq expression
gnmic getset -a <ip:port> \
    --get /interface/ifindex \
    --condition '.[] | .updates[].values[""]["srl_nokia-interfaces:interface"][] | select(.ifindex==70) | (.name != "" or .name !=null)' \
    --update 'jq(.[] | .updates[].values[""]["srl_nokia-interfaces:interface"][] | select(.ifindex==70) | "interface[name=" + .name + "]/admin-state")' \
    --value enable