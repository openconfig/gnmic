
### Description

The set-request sub command generates a Set request file given a list of update and/or replace paths.

If no paths are supplied, a root (`/`) replace path is used as a default.

The generated file can be manually edited and used with `gnmic` set command:

`gnmic set --request-file <path_to_generated_file>`

Aliases: `sreq`, `srq`, `sr`

### Usage

`gnmic [global-flags] generate [generate-flags] set-request [sub-command-flags]`

### Flags

#### update

The `--update` flag specifies a valid xpath, used to generate an __updates__ section of the [set request file](../set.md#template-based-set-request).

Multiple `--update` flags can be supplied.

#### replace

The `--replace` flag specifies a valid xpath, used to generate a __replaces__ section of the [set request file](../set.md#template-based-set-request).

Multiple `--replace` flags can be supplied.

### Examples

#### Openconfig

YANG repo: [openconfig/public](https://github.com/openconfig/public)

Clone the OpenConfig repository:

```bash
git clone https://github.com/openconfig/public
cd public
```

```bash
gnmic --encoding json_ietf \
          generate  \
          --file release/models \
          --dir third_party \
          --exclude ietf-interfaces \
          set-request \
          --replace /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address
```

The above command generates the below YAML output (JSON if `--json` flag is supplied)

```yaml
replaces:
- path: /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address
  value:
  - config:
      ip: ""
      prefix-length: ""
    ip: ""
    vrrp:
      vrrp-group:
      - config:
          accept-mode: "false"
          advertisement-interval: "100"
          preempt: "true"
          preempt-delay: "0"
          priority: "100"
          virtual-address: ""
          virtual-router-id: ""
        interface-tracking:
          config:
            priority-decrement: "0"
            track-interface: ""
        virtual-router-id: ""
  encoding: JSON_IETF
```

The __value__ section can be filled with the desired configuration variables.


#### Nokia SR OS

```bash
git clone https://github.com/nokia/7x50_YangModels
cd 7x50_YangModels
git checkout sros_21.2.r2
```

```bash
gnmic generate \
        --file YANG/nokia-combined \
        --dir YANG \
        set-request \
        --replace /configure/service/vprn/bgp/family
```

The above command generates the below YAML output (JSON if `--json` flag is supplied)

```yaml
replaces:
- path: /configure/service/vprn/bgp/family
  value:
    flow-ipv4: "false"
    flow-ipv6: "false"
    ipv4: "true"
    ipv6: "false"
    label-ipv4: "false"
    mcast-ipv4: "false"
    mcast-ipv6: "false"
```

#### Cisco

YANG repo: [YangModels/yang](https://github.com/YangModels/yang)

Clone the `YangModels/yang` repo and change into the main directory of the repo:

```bash
git clone https://github.com/YangModels/yang
cd yang/vendor
```

```bash
gnmic --encoding json_ietf \
          generate  \
          --file vendor/cisco/xr/721/Cisco-IOS-XR-um-router-bgp-cfg.yang \
          --file vendor/cisco/xr/721/Cisco-IOS-XR-ipv4-bgp-oper.yang \
          --dir standard/ietf \
          set-request \
          --path /active-nodes
```

The above command generates the below YAML output (JSON if `--json` flag is supplied)

```yaml
replaces:
- path: /active-nodes
  value:
    active-node:
    - node-name: ""
      selective-vrf-download:
        role:
          address-family:
            ipv4:
              unicast: ""
            ipv6:
              unicast: ""
        vrf-groups:
          vrf-group:
          - vrf-group-name: ""
  encoding: JSON_IETF
```

#### Juniper

YANG repo: [Juniper/yang](https://github.com/Juniper/yang)

Clone the Juniper YANG repository and change into the release directory:

```bash
git clone https://github.com/Juniper/yang
cd yang/20.3/20.3R1
```

```bash
gnmic --encoding json_ietf \
          generate
          --file junos/conf \
          --dir common 
          set-request \
          --replace /configuration/interfaces/interface/unit/family/inet/address
```

The above command generates the below YAML output (JSON if `--json` flag is supplied)

```yaml
replaces:
- path: /configuration/interfaces/interface/unit/family/inet/address
  value:
  - apply-groups: ""
    apply-groups-except: ""
    apply-macro:
    - data:
      - name: ""
        value: ""
      name: ""
    arp:
    - case_1: ""
      case_2: ""
      l2-interface: ""
      name: ""
      publish: ""
    broadcast: ""
    destination: ""
    destination-profile: ""
    master-only: ""
    multipoint-destination:
    - apply-groups: ""
      apply-groups-except: ""
      apply-macro:
      - data:
        - name: ""
          value: ""
        name: ""
      case_1: ""
      case_2: ""
      epd-threshold:
        apply-groups: ""
        apply-groups-except: ""
        apply-macro:
        - data:
          - name: ""
            value: ""
          name: ""
        epd-threshold-plp0: ""
        plp1: ""
      inverse-arp: ""
      name: ""
      oam-liveness:
        apply-groups: ""
        apply-groups-except: ""
        apply-macro:
        - data:
          - name: ""
            value: ""
          name: ""
        down-count: ""
        up-count: ""
      oam-period:
        disable: {}
        oam_period: ""
      shaping:
        apply-groups: ""
        apply-groups-except: ""
        apply-macro:
        - data:
          - name: ""
            value: ""
          name: ""
        cbr:
          cbr-value: ""
          cdvt: ""
        queue-length: ""
        rtvbr:
          burst: ""
          cdvt: ""
          peak: ""
          sustained: ""
        vbr:
          burst: ""
          cdvt: ""
          peak: ""
          sustained: ""
      transmit-weight: ""
    name: ""
    preferred: ""
    primary: ""
    virtual-gateway-address: ""
    vrrp-group:
    - advertisements-threshold: ""
      apply-groups: ""
      apply-groups-except: ""
      apply-macro:
      - data:
        - name: ""
          value: ""
        name: ""
      authentication-key: ""
      authentication-type: ""
      case_1: ""
      case_2: ""
      case_3: ""
      name: ""
      preferred: ""
      priority: ""
      track:
        apply-groups: ""
        apply-groups-except: ""
        apply-macro:
        - data:
          - name: ""
            value: ""
          name: ""
        interface:
        - apply-groups: ""
          apply-groups-except: ""
          apply-macro:
          - data:
            - name: ""
              value: ""
            name: ""
          bandwidth-threshold:
          - name: ""
            priority-cost: ""
          name: ""
          priority-cost: ""
        priority-hold-time: ""
        route:
        - priority-cost: ""
          route_address: ""
          routing-instance: ""
      virtual-link-local-address: ""
      vrrp-inherit-from:
        active-group: ""
        active-interface: ""
        apply-groups: ""
        apply-groups-except: ""
        apply-macro:
        - data:
          - name: ""
            value: ""
          name: ""
    web-authentication:
      apply-groups: ""
      apply-groups-except: ""
      apply-macro:
      - data:
        - name: ""
          value: ""
        name: ""
      http: ""
      https: ""
      redirect-to-https: ""
  encoding: JSON_IETF
```

#### Arista

YANG repo: [aristanetworks/yang](https://github.com/aristanetworks/yang)

Arista uses a subset of OpenConfig modules and does not provide IETF modules inside their repo. So make sure you have IETF models available so you can reference it, a `openconfig/public` is a good candidate.

Clone the Arista YANG repo:

```bash
git clone https://github.com/aristanetworks/yang
cd yang
```

The above command generates the below YAML output (JSON if `--json` flag is supplied)

```bash
gnmic --encoding json_ietf \
          generate
          --file EOS-4.23.2F/openconfig/public/release/models \
          --dir ../openconfig/public/third_party/ietf \
          --exclude ietf-interfaces \
          set-request \
          --replace bgp/neighbors/neighbor/config
```

```yaml
replaces:
- path: bgp/neighbors/neighbor/config
  value:
    auth-password: ""
    description: ""
    enabled: "true"
    local-as: ""
    neighbor-address: ""
    peer-as: ""
    peer-group: ""
    peer-type: ""
    remove-private-as: ""
    route-flap-damping: "false"
    send-community: NONE
```
