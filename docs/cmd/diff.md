
### Description

The `diff` command is similar to a `get` or `subscribe` (mode ONCE) commands ran against at least 2 targets, a reference and one or more compared targets.
The command will compare the returned responses from the compared targets to the ones returned from the reference target and only print the difference between them.

The output is printed as a list "flattened" gNMI updates, each line containing an XPath pointing to a leaf followed by its value.

Each line is preceded with either signs `+` or `-`:

- `+` means the leaf and its value are present in the compared target but not in the reference target.
- `-` means the leaf and its value are present in the reference target but not in the compared target.

e.g:

```text
+	network-instance[name=default]/interface[name=ethernet-1/36.0]: {}
-	network-instance[name=default]/protocols/bgp/autonomous-system: 101
```

The output above indicates:

- The compared target has interface `ethernet-1/36.0` added to network instance `default` while the reference doesn't.
- The compared target is missing the autonomous-system `101` configuration under network-instance `default` protocols/bgp compared to the reference.

The data to be compared is specified with the flag `--path`, which can be set multiple times to compare multiple data sets.
By default, the data it is retrieved using a `Get RPC`, if the flag `--sub` is present, a `Subscribe RPC` with mode ONCE is used instead.

Each of the `get` and `subscribe` methods has pros and cons, with the `get` method you can choose to compare `CONFIG` or `STATE` only, via the flag `--type`.
The `subscribe` method allows to stream the response(s) in case a larger data set needs to be compared. In addition to that, some routers support more encoding options when using the `subscribe RPC`

Multiple targets can be compared to the reference at once, the printed output of each difference will start with the line `"$reference" vs "$compared"`

Aliases: `compare`

### Usage

`gnmic [global-flags] diff [local-flags]`

### Flags

#### ref

The `--ref` flag is a mandatory flag that specifies the target to used as reference to compare other targets to.

#### compare

The `--compare` flag is a mandatory flag that specifies the targets to compare to the reference target.

#### prefix

As per [path prefixes](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#241-path-prefixes), the prefix `[--prefix]` flag represents a common prefix that is applied to all paths specified using the local `--path` flag. Defaults to `""`.

#### path

The mandatory path flag `[--path]` is used to specify the [path(s)](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#222-paths) the client wants to receive a snapshot of.

Multiple paths can be specified by using multiple `--path` flags:

```bash
gnmic --insecure \
      --ref router1
      --compare router2,router3
      diff --path "/state/ports[port-id=*]" \
           --path "/state/router[router-name=*]/interface[interface-name=*]"
```

If a user needs to provide [origin](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#222-paths) information to the Path message, the following pattern should be used for the path string: `"origin:path"`:

#### model

The optional model flag `[--model]` is used to specify the schema definition modules that the target should use when returning a GetResponse. The model name should match the names returned in Capabilities RPC. Currently only single model name is supported.

#### target

With the optional `[--target]` flag it is possible to supply the [path target](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#2221-path-target) information in the prefix field of the GetRequest message.

#### type

The type flag `[--type]` is used to specify the [data type](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L399) requested from the server.

One of:  ALL, CONFIG, STATE, OPERATIONAL (defaults to "ALL")

#### sub

When the flag `--sub` is present, `gnmic` will use a `Subscribe RPC` with mode ONCE, instead of a `Get RPC` to retrieve the data to be compared.

### Examples

```bash
gnmic diff -t config --skip-verify -e ascii \
           --ref clab-te-leaf1 \
           --compare clab-te-leaf2 \
           --path /network-instance
```

```bash
"clab-te-leaf1:57400" vs "clab-te-leaf2:57400"
+	network-instance[name=default]/interface[name=ethernet-1/36.0]                                    : {}
-	network-instance[name=default]/protocols/bgp/autonomous-system                                    : 101
+	network-instance[name=default]/protocols/bgp/autonomous-system                                    : 102
-	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:11:1]            : {}
-	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:11:1]/admin-state: enable
-	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:11:1]/peer-as    : 201
-	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:11:1]/peer-group : eBGPv6
-	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:12:1]            : {}
-	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:12:1]/admin-state: enable
-	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:12:1]/peer-as    : 202
-	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:12:1]/peer-group : eBGPv6
+	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:21:1]            : {}
+	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:21:1]/admin-state: enable
+	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:21:1]/peer-as    : 201
+	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:21:1]/peer-group : eBGPv6
+	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:22:1]            : {}
+	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:22:1]/admin-state: enable
+	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:22:1]/peer-as    : 202
+	network-instance[name=default]/protocols/bgp/neighbor[peer-address=2002::192:168:22:1]/peer-group : eBGPv6
+	network-instance[name=default]/protocols/bgp/router-id                                            : 10.0.1.2
-	network-instance[name=default]/protocols/bgp/router-id                                            : 10.0.1.1
-	network-instance[name=myins]                                                                      : {}
-	network-instance[name=myins]/admin-state                                                          : enable
-	network-instance[name=myins]/description                                                          : desc1
-	network-instance[name=myins]/interface[name=ethernet-1/36.0]                                      : {}
-	network-instance[name=myins]/type                                                                 : ip-vrf
```
