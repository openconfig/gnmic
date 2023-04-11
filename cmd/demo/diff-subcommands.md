## Usage

```bash
$ gnmic diff setrequest setrequest.textproto setrequest2.textproto

SetRequestIntentDiff(-A, +B):
-------- deletes/replaces --------
+ /network-instances/network-instance[name=VrfBlue]: deleted or replaced only in B
-------- updates --------
m /system/config/hostname:
  - "violetsareblue"
  + "rosesarered"

$ gnmic diff set-to-notifs setrequest.textproto notifs.textproto

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

$ gnmic diff set-to-notifs setrequest.textproto getresponse.textproto

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
