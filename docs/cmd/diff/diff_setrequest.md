### Description

The `diff setrequest` command is used to compare the intent between two
`SetRequest` messages encoded in textproto format.

The output is printed as a list of "flattened" gNMI updates, each line
containing an XPath pointing to a leaf followed by its value.

Each line is preceded with either signs `+` or `-`:

-   `+` means the leaf and its value are present in the new SetRequest but not
    in the reference SetRequest.
-   `-` means the leaf and its value are present in the reference SetRequest but
    not in the new SetRequest.

e.g:

```text
SetRequestIntentDiff(-A, +B):
-------- deletes/replaces --------
+ /network-instances/network-instance[name=VrfBlue]: deleted or replaced only in B
-------- updates --------
m /system/config/hostname:
  - "violetsareblue"
  + "rosesarered"
```

The output above indicates:

-   The new target deletes or replaces the path
    `/network-instances/network-instance[name=VrfBlue]` while the reference
    doesn't.
-   The new target changes the value of `/system/config/hostname` compared to
    the reference from `"violetsareblue"` to `"rosesarered"`.

When `--full` is specified, values common between the two SetRequest are also
shown.

### SetRequest Intent

It is possible for two SetRequests to be different but which are semantically
equivalent -- i.e. they both modify the same leafs in the same ways. In other
words, their overall effects are the same.

For example, a replace on the leaf `/system/config/hostname` with the value
`"foo"` is the same as an update on the same leaf with the same value. A replace
on the container `/system/` with the value `{ config: { hostname: "foo" } }` is
the same as a delete on that container followed by a replace to the leaf.
Overwrites are also possible, although this is currently unsupported.

In order to compare equivalent SetRequests correctly, this tool breaks down a
SetRequest into its "minimal intent" (deletes followed by updates) prior to the
diff computation. This is why the output groups deletes/replaces in the same
section.

### Usage

`gnmic [global-flags] diff setrequest [local-flags]`

### Flags

#### ref

The `--ref` flag is a mandatory flag that specifies the reference gNMI
SetRequest textproto file for comparing against the new SetRequest.

#### new

The `--new` flag is a mandatory flag that specifies the new gNMI SetRequest
textproto file for comparing against the reference SetRequest.

### Examples

```bash
$ gnmic diff setrequest --ref cmd/demo/setrequest.textproto --new cmd/demo/setrequest2.textproto
```
