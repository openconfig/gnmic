### Description

The `diff set-to-notifs` command is used to verify whether a set of
notifications from a `GetResponse` or a stream of `SubscribeResponse` messages
comply with a `SetRequest` messages in textproto format. The envisioned use case
is to check whether a stored snapshot of device state matches that of the
intended state as specified by a `SetRequest`.

The output is printed as a list of "flattened" gNMI updates, each line
containing an XPath pointing to a leaf followed by its value.

Each line is preceded with either signs `+` or `-`:

-   `+` means the leaf and its value are present in the new SetRequest but not
    in the reference SetRequest.
-   `-` means the leaf and its value are present in the reference SetRequest but
    not in the new SetRequest.

e.g:

```text
SetToNotifsDiff(-want/SetRequest, +got/Notifications):
- /lacp/interfaces/interface[name=Port-Channel9]/config/interval: "FAST"
- /lacp/interfaces/interface[name=Port-Channel9]/config/name: "Port-Channel9"
- /lacp/interfaces/interface[name=Port-Channel9]/name: "Port-Channel9"
- /network-instances/network-instance[name=VrfBlue]/config/name: "VrfBlue"
- /network-instances/network-instance[name=VrfBlue]/config/type: "openconfig-network-instance-types:L3VRF"
- /network-instances/network-instance[name=VrfBlue]/name: "VrfBlue"
m /system/config/hostname:
  - "violetsareblue"
  + "rosesarered"
```

The output above indicates:

-   The set of paths starting with
    `/lacp/interfaces/interface[name=Port-Channel9]/config/interval: "FAST"` are
    present in the SetRequest but missing in the response from the device.
-   The value at path `/system/config/hostname` does not match that of the
    SetRequest.

When `--full` is specified, values common between the SetRequest and the
response messages are also shown.

### How to obtain a GetResponse or SubscribeResponse

To obtain GetRespnse/SubscribeResponse in textproto format, simply run `gnmic`'s
subscribe or get functions and pass in the flag `--format prototext`.

Responses retrieved from either GetRequest or SubscribeRequest are supported by
this command's `--response` flag.

### Usage

`gnmic [global-flags] diff set-to-notifs [local-flags]`

### Flags

#### setrequest

The `--setrequest` flag is a mandatory flag that specifies the reference gNMI
SetRequest textproto file for comparing against the new SetRequest.

#### response

The `--response` flag is a mandatory flag that specifies the gNMI Notifications
textproto file (can contain a GetResponse or SubscribeResponse stream) for
comparing against the reference SetRequest.

### Examples

```bash
$ gnmic diff set-to-notifs --setrequest cmd/demo/setrequest.textproto --response cmd/demo/subscriberesponses.textproto
```
