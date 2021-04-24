
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

The `--update` flag specifies a valid xpath to be used in the __updates__ section of the [set request file](../set.md#template-based-set-request).

Multiple `--update` flags can be supplied.

#### replace

The `--replace` flag specifies a valid xpath to be used in the __replaces__ section of the [set request file](../set.md#template-based-set-request).

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
          --replace /interfaces/interface
```

The above command generates the below YAML output (JSON if `--json` flag is supplied)

??? note "YAML"
    ```yaml
      replaces:
      - path: /interfaces/interface:
        value:
        - aggregation:
            config:
              lag-type: ""
              min-links: ""
            switched-vlan:
              config:
                access-vlan: ""
                interface-mode: ""
                native-vlan: ""
                trunk-vlans: ""
          config:
            description: ""
            enabled: "true"
            loopback-mode: "false"
            mtu: ""
            name: ""
            tpid: oc-vlan-types:TPID_0X8100
            type: ""
          ethernet:
            authenticated-sessions: {}
            config:
              aggregate-id: ""
              auto-negotiate: "true"
              duplex-mode: ""
              enable-flow-control: "false"
              mac-address: ""
              port-speed: ""
            dot1x:
              config:
                auth-fail-vlan: ""
                authenticate-port: ""
                host-mode: ""
                max-requests: ""
                reauthenticate-interval: ""
                retransmit-interval: ""
                server-fail-vlan: ""
                supplicant-timeout: ""
            poe:
              config:
                enabled: "true"
            switched-vlan:
              config:
                access-vlan: ""
                interface-mode: ""
                native-vlan: ""
                trunk-vlans: ""
              dot1x-vlan-map:
                vlan-name:
                - config:
                    id: ""
                    vlan-name: ""
                  vlan-name: ""
          hold-time:
            config:
              down: "0"
              up: "0"
          name: ""
          routed-vlan:
            config:
              vlan: ""
            ipv4:
              addresses:
                address:
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
              config:
                dhcp-client: "false"
                enabled: "true"
                mtu: ""
              neighbors:
                neighbor:
                - config:
                    ip: ""
                    link-layer-address: ""
                  ip: ""
              proxy-arp:
                config:
                  mode: DISABLE
              unnumbered:
                config:
                  enabled: "false"
                interface-ref:
                  config:
                    interface: ""
                    subinterface: ""
            ipv6:
              addresses:
                address:
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
                        virtual-link-local: ""
                        virtual-router-id: ""
                      interface-tracking:
                        config:
                          priority-decrement: "0"
                          track-interface: ""
                      virtual-router-id: ""
              config:
                dhcp-client: "false"
                dup-addr-detect-transmits: "1"
                enabled: "true"
                mtu: ""
              neighbors:
                neighbor:
                - config:
                    ip: ""
                    link-layer-address: ""
                  ip: ""
              router-advertisement:
                config:
                  interval: ""
                  lifetime: ""
                  suppress: "false"
              unnumbered:
                config:
                  enabled: "false"
                interface-ref:
                  config:
                    interface: ""
                    subinterface: ""
          sonet: {}
          subinterfaces:
            subinterface:
            - config:
                description: ""
                enabled: "true"
                index: "0"
              index: ""
              ipv4:
                addresses:
                  address:
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
                config:
                  dhcp-client: "false"
                  enabled: "true"
                  mtu: ""
                neighbors:
                  neighbor:
                  - config:
                      ip: ""
                      link-layer-address: ""
                    ip: ""
                proxy-arp:
                  config:
                    mode: DISABLE
                unnumbered:
                  config:
                    enabled: "false"
                  interface-ref:
                    config:
                      interface: ""
                      subinterface: ""
              ipv6:
                addresses:
                  address:
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
                          virtual-link-local: ""
                          virtual-router-id: ""
                        interface-tracking:
                          config:
                            priority-decrement: "0"
                            track-interface: ""
                        virtual-router-id: ""
                autoconf:
                  config:
                    create-global-addresses: "true"
                    create-temporary-addresses: "false"
                    temporary-preferred-lifetime: "86400"
                    temporary-valid-lifetime: "604800"
                config:
                  dhcp-client: "false"
                  dup-addr-detect-transmits: "1"
                  enabled: "true"
                  mtu: ""
                neighbors:
                  neighbor:
                  - config:
                      ip: ""
                      link-layer-address: ""
                    ip: ""
                router-advertisement:
                  config:
                    interval: ""
                    lifetime: ""
                    suppress: "false"
                unnumbered:
                  config:
                    enabled: "false"
                  interface-ref:
                    config:
                      interface: ""
                      subinterface: ""
              vlan:
                config:
                  vlan-id: ""
                egress-mapping:
                  config:
                    tpid: ""
                    vlan-id: ""
                    vlan-stack-action: ""
                ingress-mapping:
                  config:
                    tpid: ""
                    vlan-id: ""
                    vlan-stack-action: ""
                match:
                  double-tagged:
                    config:
                      inner-vlan-id: ""
                      outer-vlan-id: ""
                  double-tagged-inner-list:
                    config:
                      inner-vlan-ids: ""
                      outer-vlan-id: ""
                  double-tagged-inner-outer-range:
                    config:
                      inner-high-vlan-id: ""
                      inner-low-vlan-id: ""
                      outer-high-vlan-id: ""
                      outer-low-vlan-id: ""
                  double-tagged-inner-range:
                    config:
                      inner-high-vlan-id: ""
                      inner-low-vlan-id: ""
                      outer-vlan-id: ""
                  double-tagged-outer-list:
                    config:
                      inner-vlan-id: ""
                      outer-vlan-ids: ""
                  double-tagged-outer-range:
                    config:
                      inner-vlan-id: ""
                      outer-high-vlan-id: ""
                      outer-low-vlan-id: ""
                  single-tagged:
                    config:
                      vlan-id: ""
                  single-tagged-list:
                    config:
                      vlan-ids: ""
                  single-tagged-range:
                    config:
                      high-vlan-id: ""
                      low-vlan-id: ""
          tunnel:
            config:
              dst: ""
              gre-key: ""
              src: ""
              ttl: ""
            ipv4:
              addresses:
                address:
                - config:
                    ip: ""
                    prefix-length: ""
                  ip: ""
              config:
                dhcp-client: "false"
                enabled: "true"
                mtu: ""
              neighbors:
                neighbor:
                - config:
                    ip: ""
                    link-layer-address: ""
                  ip: ""
              proxy-arp:
                config:
                  mode: DISABLE
              unnumbered:
                config:
                  enabled: "false"
                interface-ref:
                  config:
                    interface: ""
                    subinterface: ""
            ipv6:
              addresses:
                address:
                - config:
                    ip: ""
                    prefix-length: ""
                  ip: ""
              config:
                dhcp-client: "false"
                dup-addr-detect-transmits: "1"
                enabled: "true"
                mtu: ""
              neighbors:
                neighbor:
                - config:
                    ip: ""
                    link-layer-address: ""
                  ip: ""
              router-advertisement:
                config:
                  interval: ""
                  lifetime: ""
                  suppress: "false"
              unnumbered:
                config:
                  enabled: "false"
                interface-ref:
                  config:
                    interface: ""
                    subinterface: ""
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
        --replace /configure/service/vprn
```

??? note "YAML"
    ```yaml
    replaces:
    - path: /configure/service/vprn
      value:
      - aaa:
          remote-servers:
            radius:
              access-algorithm: direct
              accounting: "false"
              accounting-port: "1813"
              admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              authorization: "false"
              interactive-authentication: "false"
              port: "1812"
              server:
              - address: ""
                apply-groups: ""
                apply-groups-exclude: ""
                index: ""
                secret: ""
              server-retry: "3"
              server-timeout: "3"
              use-default-template: "false"
            tacplus:
              accounting:
                record-type: stop-only
              admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              authorization:
                use-priv-lvl: "false"
              interactive-authentication: "false"
              priv-lvl-map:
                priv-lvl:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  level: ""
                  user-profile-name: ""
              server:
              - address: ""
                apply-groups: ""
                apply-groups-exclude: ""
                index: ""
                port: "49"
                secret: ""
              server-timeout: "3"
              use-default-template: "true"
        admin-state: disable
        aggregates:
          aggregate:
          - aggregator:
              address: ""
              as-number: ""
            apply-groups: ""
            apply-groups-exclude: ""
            as-set: "false"
            community: ""
            description: ""
            discard-component-communities: "false"
            indirect: ""
            ip-prefix: ""
            local-preference: ""
            policy: ""
            summary-only: "false"
            tunnel-group: ""
          apply-groups: ""
          apply-groups-exclude: ""
        allow-export-bgp-vpn: "false"
        apply-groups: ""
        apply-groups-exclude: ""
        auto-bind-tunnel:
          allow-flex-algo-fallback: "false"
          apply-groups: ""
          apply-groups-exclude: ""
          ecmp: "1"
          enforce-strict-tunnel-tagging: "false"
          resolution: none
          resolution-filter:
            bgp: "true"
            gre: "false"
            ldp: "false"
            mpls-fwd-policy: "false"
            rib-api: "false"
            rsvp: "false"
            sr-isis: "false"
            sr-ospf: "false"
            sr-ospf3: "false"
            sr-policy: "false"
            sr-te: "false"
            udp: "false"
          weighted-ecmp: "false"
        autonomous-system: ""
        bgp:
          admin-state: enable
          advertise-inactive: "false"
          advertise-ipv6-next-hops:
            ipv4: "false"
          aggregator-id-zero: "false"
          apply-groups: ""
          apply-groups-exclude: ""
          asn-4-byte: "true"
          authentication-key: ""
          authentication-keychain: ""
          backup-path:
            ipv4: "false"
            ipv6: "false"
            label-ipv4: "false"
          best-path-selection:
            always-compare-med:
              med-value: "off"
              strict-as: "true"
            as-path-ignore:
              ipv4: "false"
              ipv6: "false"
              label-ipv4: "false"
            compare-origin-validation-state: "false"
            deterministic-med: "false"
            ebgp-ibgp-equal:
              ipv4: "false"
              ipv6: "false"
              label-ipv4: "false"
            ignore-nh-metric: "false"
            ignore-router-id: {}
            origin-invalid-unusable: "false"
          bfd-liveness: "false"
          client-reflect: "true"
          cluster:
            cluster-id: ""
          connect-retry: "120"
          convergence:
            family:
            - apply-groups: ""
              apply-groups-exclude: ""
              family-type: ""
              max-wait-to-advertise: "0"
            min-wait-to-advertise: "0"
          damp-peer-oscillations:
            error-interval: "30"
            idle-hold-time:
              initial-wait: "0"
              max-wait: "60"
              second-wait: "5"
          damping: "false"
          default-label-preference:
            ebgp: "0"
            ibgp: "0"
          default-preference:
            ebgp: "0"
            ibgp: "0"
          description: ""
          dynamic-neighbor-limit: ""
          ebgp-default-reject-policy:
            export: "true"
            import: "true"
          eibgp-loadbalance: "false"
          enforce-first-as: "false"
          error-handling:
            update-fault-tolerance: "false"
          export:
            apply-groups: ""
            apply-groups-exclude: ""
            policy: ""
          extended-nh-encoding:
            ipv4: "false"
          family:
            flow-ipv4: "false"
            flow-ipv6: "false"
            ipv4: "true"
            ipv6: "false"
            label-ipv4: "false"
            mcast-ipv4: "false"
            mcast-ipv6: "false"
          fast-external-failover: "true"
          flowspec:
            validate-dest-prefix: "false"
            validate-redirect-ip: "false"
          graceful-restart:
            gr-notification: "false"
            long-lived:
              advertise-stale-to-all-neighbors: "false"
              advertised-stale-time: "86400"
              family:
              - advertised-stale-time: "86400"
                apply-groups: ""
                apply-groups-exclude: ""
                family-type: ""
                helper-override-stale-time: ""
              forwarding-bits-set: none
              helper-override-restart-time: ""
              helper-override-stale-time: ""
              without-no-export: "false"
            restart-time: "120"
            stale-routes-time: "360"
          group:
          - admin-state: enable
            advertise-inactive: ""
            advertise-ipv6-next-hops:
              ipv4: "false"
            aggregator-id-zero: ""
            apply-groups: ""
            apply-groups-exclude: ""
            as-override: "false"
            asn-4-byte: ""
            authentication-key: ""
            authentication-keychain: ""
            bfd-liveness: ""
            capability-negotiation: "true"
            client-reflect: ""
            cluster:
              cluster-id: ""
            connect-retry: ""
            damp-peer-oscillations:
              error-interval: "30"
              idle-hold-time:
                initial-wait: "0"
                max-wait: "60"
                second-wait: "5"
            damping: ""
            default-label-preference:
              ebgp: ""
              ibgp: ""
            default-preference:
              ebgp: ""
              ibgp: ""
            description: ""
            dynamic-neighbor:
              match:
                prefix:
                - allowed-peer-as: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  ip-prefix: ""
            dynamic-neighbor-limit: ""
            ebgp-default-reject-policy:
              export: "true"
              import: "true"
            enforce-first-as: ""
            error-handling:
              update-fault-tolerance: ""
            export:
              apply-groups: ""
              apply-groups-exclude: ""
              policy: ""
            extended-nh-encoding:
              ipv4: "false"
            family:
              flow-ipv4: "false"
              flow-ipv6: "false"
              ipv4: "false"
              ipv6: "false"
              label-ipv4: "false"
              mcast-ipv4: "false"
              mcast-ipv6: "false"
            fast-external-failover: ""
            graceful-restart:
              gr-notification: "false"
              long-lived:
                advertise-stale-to-all-neighbors: "false"
                advertised-stale-time: "86400"
                family:
                - advertised-stale-time: "86400"
                  apply-groups: ""
                  apply-groups-exclude: ""
                  family-type: ""
                  helper-override-stale-time: "16777216"
                forwarding-bits-set: none
                helper-override-restart-time: ""
                helper-override-stale-time: ""
                without-no-export: "false"
              restart-time: "300"
              stale-routes-time: "360"
            group-name: ""
            hold-time:
              minimum-hold-time: "0"
              seconds: ""
            import:
              apply-groups: ""
              apply-groups-exclude: ""
              policy: ""
            initial-send-delay-zero: ""
            keepalive: ""
            label-preference: ""
            link-bandwidth:
              accept-from-ebgp:
                ipv4: "false"
                ipv6: "false"
                label-ipv4: "false"
              add-to-received-ebgp:
                ipv4: "false"
                ipv6: "false"
                label-ipv4: "false"
              aggregate-used-paths:
                ipv4: "false"
                ipv6: "false"
                label-ipv4: "false"
              send-to-ebgp:
                ipv4: "false"
                ipv6: "false"
                label-ipv4: "false"
            local-address: ""
            local-as:
              as-number: ""
              prepend-global-as: "true"
              private: "false"
            local-preference: ""
            loop-detect: ""
            loop-detect-threshold: ""
            med-out: ""
            min-route-advertisement: ""
            monitor:
              admin-state: disable
              all-stations: "false"
              apply-groups: ""
              apply-groups-exclude: ""
              route-monitoring:
                post-policy: "false"
                pre-policy: "false"
              station:
              - station-name: ""
            multihop: ""
            multipath-eligible: "false"
            next-hop-self: "false"
            origin-validation:
              ipv4: "false"
              ipv6: "false"
              label-ipv4: "false"
            passive: "false"
            path-mtu-discovery: ""
            peer-as: ""
            peer-ip-tracking: ""
            preference: ""
            prefix-limit:
            - apply-groups: ""
              apply-groups-exclude: ""
              family: ""
              idle-timeout: ""
              log-only: "false"
              maximum: ""
              post-import: "false"
              threshold: "90"
            remove-private:
              limited: "false"
              replace: "false"
              skip-peer-as: "false"
            send-communities:
              extended: ""
              large: ""
              standard: ""
            send-default:
              export-policy: ""
              ipv4: "false"
              ipv6: "false"
            split-horizon: ""
            static-group: "true"
            tcp-mss: ""
            third-party-nexthop: ""
            ttl-security: ""
            type: no-type
          hold-time:
            minimum-hold-time: "0"
            seconds: "90"
          ibgp-multipath: "false"
          import:
            apply-groups: ""
            apply-groups-exclude: ""
            policy: ""
          initial-send-delay-zero: "false"
          keepalive: "30"
          label-preference: "170"
          local-as:
            as-number: ""
            prepend-global-as: "true"
            private: "false"
          local-preference: "100"
          loop-detect: ignore-loop
          loop-detect-threshold: "0"
          med-out: ""
          min-route-advertisement: "30"
          monitor:
            admin-state: disable
            all-stations: "false"
            apply-groups: ""
            apply-groups-exclude: ""
            route-monitoring:
              post-policy: "false"
              pre-policy: "false"
            station:
            - station-name: ""
          multihop: ""
          multipath:
            ebgp: ""
            family:
            - apply-groups: ""
              apply-groups-exclude: ""
              ebgp: ""
              family-type: ""
              ibgp: ""
              max-paths: ""
              restrict: same-as-path-length
              unequal-cost: "false"
            ibgp: ""
            max-paths: "1"
            restrict: same-as-path-length
            unequal-cost: "false"
          neighbor:
          - admin-state: enable
            advertise-inactive: ""
            advertise-ipv6-next-hops:
              ipv4: "false"
            aggregator-id-zero: ""
            apply-groups: ""
            apply-groups-exclude: ""
            as-override: ""
            asn-4-byte: ""
            authentication-key: ""
            authentication-keychain: ""
            bfd-liveness: ""
            capability-negotiation: ""
            client-reflect: ""
            cluster:
              cluster-id: ""
            connect-retry: ""
            damp-peer-oscillations:
              error-interval: "30"
              idle-hold-time:
                initial-wait: "0"
                max-wait: "60"
                second-wait: "5"
            damping: ""
            default-label-preference:
              ebgp: ""
              ibgp: ""
            default-preference:
              ebgp: ""
              ibgp: ""
            description: ""
            ebgp-default-reject-policy:
              export: "true"
              import: "true"
            enforce-first-as: ""
            error-handling:
              update-fault-tolerance: ""
            export:
              apply-groups: ""
              apply-groups-exclude: ""
              policy: ""
            extended-nh-encoding:
              ipv4: "false"
            family:
              flow-ipv4: "false"
              flow-ipv6: "false"
              ipv4: "false"
              ipv6: "false"
              label-ipv4: "false"
              mcast-ipv4: "false"
              mcast-ipv6: "false"
            fast-external-failover: ""
            graceful-restart:
              gr-notification: "false"
              long-lived:
                advertise-stale-to-all-neighbors: "false"
                advertised-stale-time: "86400"
                family:
                - advertised-stale-time: "86400"
                  apply-groups: ""
                  apply-groups-exclude: ""
                  family-type: ""
                  helper-override-stale-time: "16777216"
                forwarding-bits-set: none
                helper-override-restart-time: ""
                helper-override-stale-time: ""
                without-no-export: "false"
              restart-time: "300"
              stale-routes-time: "360"
            group: ""
            hold-time:
              minimum-hold-time: "0"
              seconds: ""
            import:
              apply-groups: ""
              apply-groups-exclude: ""
              policy: ""
            initial-send-delay-zero: ""
            ip-address: ""
            keepalive: ""
            label-preference: ""
            link-bandwidth:
              accept-from-ebgp:
                ipv4: "false"
                ipv6: "false"
                label-ipv4: "false"
              add-to-received-ebgp:
                ipv4: "false"
                ipv6: "false"
                label-ipv4: "false"
              aggregate-used-paths:
                ipv4: "false"
                ipv6: "false"
                label-ipv4: "false"
              send-to-ebgp:
                ipv4: "false"
                ipv6: "false"
                label-ipv4: "false"
            local-address: ""
            local-as:
              as-number: ""
              prepend-global-as: "true"
              private: "false"
            local-preference: ""
            loop-detect: ""
            loop-detect-threshold: ""
            med-out: ""
            min-route-advertisement: ""
            monitor:
              admin-state: disable
              all-stations: "false"
              apply-groups: ""
              apply-groups-exclude: ""
              route-monitoring:
                post-policy: "false"
                pre-policy: "false"
              station:
              - station-name: ""
            multihop: ""
            multipath-eligible: ""
            next-hop-self: ""
            origin-validation:
              ipv4: "false"
              ipv6: "false"
              label-ipv4: "false"
            passive: ""
            path-mtu-discovery: ""
            peer-as: ""
            peer-creation-type: static
            peer-ip-tracking: ""
            preference: ""
            prefix-limit:
            - apply-groups: ""
              apply-groups-exclude: ""
              family: ""
              idle-timeout: ""
              log-only: "false"
              maximum: ""
              post-import: "false"
              threshold: "90"
            remove-private:
              limited: "false"
              replace: "false"
              skip-peer-as: "false"
            send-communities:
              extended: ""
              large: ""
              standard: ""
            send-default:
              export-policy: ""
              ipv4: "false"
              ipv6: "false"
            split-horizon: ""
            tcp-mss: ""
            third-party-nexthop: ""
            ttl-security: ""
            type: ""
          next-hop-resolution:
            policy: ""
            use-bgp-routes: "false"
          path-mtu-discovery: "false"
          peer-ip-tracking: "false"
          peer-tracking-policy: ""
          preference: "170"
          rapid-withdrawal: "false"
          remove-private:
            limited: "false"
            replace: "false"
            skip-peer-as: "false"
          rib-management:
            ipv4:
              leak-import:
                apply-groups: ""
                apply-groups-exclude: ""
                policy: ""
              route-table-import:
                apply-groups: ""
                apply-groups-exclude: ""
                policy-name: ""
            ipv6:
              leak-import:
                apply-groups: ""
                apply-groups-exclude: ""
                policy: ""
              route-table-import:
                apply-groups: ""
                apply-groups-exclude: ""
                policy-name: ""
            label-ipv4:
              leak-import:
                apply-groups: ""
                apply-groups-exclude: ""
                policy: ""
              route-table-import:
                apply-groups: ""
                apply-groups-exclude: ""
                policy-name: ""
          router-id: ""
          send-communities:
            extended: "true"
            large: "true"
            standard: "true"
          send-default:
            export-policy: ""
            ipv4: "false"
            ipv6: "false"
          split-horizon: "false"
          tcp-mss: ""
          third-party-nexthop: "false"
        bgp-evpn:
          mpls:
          - admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            auto-bind-tunnel:
              allow-flex-algo-fallback: "false"
              ecmp: "1"
              enforce-strict-tunnel-tagging: "false"
              resolution: none
              resolution-filter:
                bgp: "false"
                ldp: "false"
                mpls-fwd-policy: "false"
                rib-api: "false"
                rsvp: "false"
                sr-isis: "false"
                sr-ospf: "false"
                sr-ospf3: "false"
                sr-policy: "false"
                sr-te: "false"
                udp: "false"
            bgp-instance: ""
            default-route-tag: ""
            route-distinguisher: ""
            send-tunnel-encap:
              mpls: "true"
              mpls-over-udp: "false"
            vrf-export:
              apply-groups: ""
              apply-groups-exclude: ""
              policy: ""
            vrf-import:
              apply-groups: ""
              apply-groups-exclude: ""
              policy: ""
            vrf-target:
              community: ""
              export-community: ""
              import-community: ""
        bgp-ipvpn:
          mpls:
            admin-state: disable
            auto-bind-tunnel:
              allow-flex-algo-fallback: "false"
              apply-groups: ""
              apply-groups-exclude: ""
              ecmp: "1"
              enforce-strict-tunnel-tagging: "false"
              resolution: none
              resolution-filter:
                bgp: "true"
                gre: "false"
                ldp: "false"
                mpls-fwd-policy: "false"
                rib-api: "false"
                rsvp: "false"
                sr-isis: "false"
                sr-ospf: "false"
                sr-ospf3: "false"
                sr-policy: "false"
                sr-te: "false"
                udp: "false"
              weighted-ecmp: "false"
            route-distinguisher: ""
            vrf-export:
              apply-groups: ""
              apply-groups-exclude: ""
              policy: ""
            vrf-import:
              apply-groups: ""
              apply-groups-exclude: ""
              policy: ""
            vrf-target:
              community: ""
              export-community: ""
              import-community: ""
        bgp-shared-queue:
          cir: "4000"
          pir: "4000"
        bgp-vpn-backup:
          ipv4: "false"
          ipv6: "false"
        carrier-carrier-vpn: "false"
        class-forwarding: "false"
        confederation:
          confed-as-num: ""
          members:
          - as-number: ""
        customer: ""
        description: ""
        dhcp-server:
          apply-groups: ""
          apply-groups-exclude: ""
          dhcpv4:
          - admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            failover:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              ignore-mclt-on-takeover: "false"
              maximum-client-lead-time: "600"
              partner-down-delay: "86399"
              peer:
              - address: ""
                apply-groups: ""
                apply-groups-exclude: ""
                sync-tag: ""
              startup-wait-time: "120"
            force-renews: "false"
            lease-hold:
              additional-scenarios:
                internal-lease-ipsec: "false"
                solicited-release: "false"
              time: ""
            name: ""
            pool:
            - apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              failover:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                ignore-mclt-on-takeover: "false"
                maximum-client-lead-time: "600"
                partner-down-delay: "86399"
                peer:
                - address: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  sync-tag: ""
                startup-wait-time: "120"
              max-lease-time: "864000"
              min-lease-time: "600"
              minimum-free:
                absolute: "1"
                event-when-depleted: "false"
                percent: "1"
              nak-non-matching-subnet: "false"
              offer-time: "60"
              options:
                option:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  ascii-string: ""
                  duration: ""
                  empty: ""
                  hex-string: ""
                  ipv4-address: ""
                  netbios-node-type: ""
                  number: ""
              pool-name: ""
              subnet:
              - address-range:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  end: ""
                  failover-control-type: local
                  start: ""
                apply-groups: ""
                apply-groups-exclude: ""
                drain: "false"
                exclude-addresses:
                - end: ""
                  start: ""
                ipv4-prefix: ""
                maximum-declined: "64"
                minimum-free:
                  absolute: "1"
                  event-when-depleted: "false"
                  percent: "1"
                options:
                  option:
                  - apply-groups: ""
                    apply-groups-exclude: ""
                    ascii-string: ""
                    duration: ""
                    empty: ""
                    hex-string: ""
                    ipv4-address: ""
                    netbios-node-type: ""
                    number: ""
            pool-selection:
              use-gi-address:
                scope: subnet
              use-pool-from-client:
                delimiter: ""
            user-db: ""
            user-identification: mac-circuit-id
          dhcpv6:
          - admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            auto-provisioned: "false"
            defaults:
              apply-groups: ""
              apply-groups-exclude: ""
              options:
                option:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  ascii-string: ""
                  domain-string: ""
                  duration: ""
                  empty: ""
                  hex-string: ""
                  ipv6-address: ""
                  number: ""
              preferred-lifetime: "3600"
              rebind-time: "2880"
              renew-time: "1800"
              valid-lifetime: "86400"
            description: ""
            failover:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              ignore-mclt-on-takeover: "false"
              maximum-client-lead-time: "600"
              partner-down-delay: "86399"
              peer:
              - address: ""
                apply-groups: ""
                apply-groups-exclude: ""
                sync-tag: ""
              startup-wait-time: "120"
            ignore-rapid-commit: "false"
            interface-id-mapping: "false"
            lease-hold:
              additional-scenarios:
                internal-lease-ipsec: "false"
                solicited-release: "false"
              time: ""
            lease-query: "false"
            name: ""
            pool:
            - apply-groups: ""
              apply-groups-exclude: ""
              delegated-prefix:
                length: "64"
                maximum: "64"
                minimum: "48"
              description: ""
              exclude-prefix:
              - ipv6-prefix: ""
              failover:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                ignore-mclt-on-takeover: "false"
                maximum-client-lead-time: "600"
                partner-down-delay: "86399"
                peer:
                - address: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  sync-tag: ""
                startup-wait-time: "120"
              options:
                option:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  ascii-string: ""
                  domain-string: ""
                  duration: ""
                  empty: ""
                  hex-string: ""
                  ipv6-address: ""
                  number: ""
              pool-name: ""
              prefix:
              - apply-groups: ""
                apply-groups-exclude: ""
                drain: "false"
                failover-control-type: local
                ipv6-prefix: ""
                options:
                  option:
                  - apply-groups: ""
                    apply-groups-exclude: ""
                    ascii-string: ""
                    domain-string: ""
                    duration: ""
                    empty: ""
                    hex-string: ""
                    ipv6-address: ""
                    number: ""
                preferred-lifetime: "3600"
                prefix-length-threshold:
                - absolute: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  event-when-depleted: "false"
                  percent: ""
                  prefix-length: ""
                prefix-type:
                  pd: "true"
                  wan-host: "true"
                rebind-time: "2880"
                renew-time: "1800"
                valid-lifetime: "86400"
              prefix-length-threshold:
              - apply-groups: ""
                apply-groups-exclude: ""
                event-when-depleted: "false"
                minimum-free-percent: "0"
                prefix-length: ""
            pool-selection:
              use-link-address:
                scope: subnet
              use-pool-from-client:
                delimiter: ""
            server-id:
              apply-groups: ""
              apply-groups-exclude: ""
              duid-enterprise:
                ascii-string: ""
                hex-string: ""
              duid-link-local: ""
            user-identification: duid
        dns:
          admin-state: disable
          apply-groups: ""
          apply-groups-exclude: ""
          default-domain: ""
          ipv4-source-address: use-interface-ip
          ipv6-source-address: use-interface-ip
          server: ""
        ecmp: "1"
        ecmp-unequal-cost: "false"
        entropy-label: "false"
        eth-cfm:
          apply-groups: ""
          apply-groups-exclude: ""
        export-inactive-bgp: "false"
        fib-priority: standard
        firewall:
          apply-groups: ""
          apply-groups-exclude: ""
          domain:
          - admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            dhcpv6-server:
              name: ""
              router-instance: ""
            name: ""
            nat: ""
            prefix:
            - apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              ip-prefix: ""
            wlan-gw: ""
        flowspec:
          apply-groups: ""
          apply-groups-exclude: ""
          filter-cam-type: normal
          ip-filter-max-size: "512"
          ipv6-filter-max-size: "512"
        grt-leaking:
          allow-local-management: "false"
          apply-groups: ""
          apply-groups-exclude: ""
          export-grt:
            policy-name: ""
          export-limit: "5"
          export-v6-limit: "5"
          grt-lookup: "false"
          import-grt:
            policy-name: ""
        gsmp:
          admin-state: disable
          apply-groups: ""
          apply-groups-exclude: ""
          group:
          - admin-state: disable
            ancp:
              dynamic-topology-discovery: "true"
              oam: "false"
            apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            hold-multiplier: "3"
            idle-filter: "false"
            keepalive: "10"
            name: ""
            neighbor:
            - admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              local-address: ""
              priority-marking:
                dscp: ""
                prec: ""
              remote-address: ""
            persistency: "false"
        gtp:
          s11:
            interface:
            - apn-policy: ""
              apply-groups: ""
              apply-groups-exclude: ""
              interface-name: ""
            peer-profile-map:
              prefix:
              - apply-groups: ""
                apply-groups-exclude: ""
                peer-prefix: ""
                peer-profile: ""
          upf-data-endpoint:
            apply-groups: ""
            apply-groups-exclude: ""
            interface: ""
          uplink:
            apn: ""
            apply-groups: ""
            apply-groups-exclude: ""
            pdn-type: ipv4
            peer-profile-map:
              prefix:
              - apply-groups: ""
                apply-groups-exclude: ""
                peer-prefix: ""
                peer-profile: ""
        hash-label: "false"
        igmp:
          admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          forwarding-group-interface:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            forwarding-service: ""
            group-interface-name: ""
            import-policy: ""
            maximum-number-group-sources: ""
            maximum-number-groups: ""
            maximum-number-sources: ""
            mcac:
              bandwidth:
                mandatory: ""
                total: ""
              interface-policy: ""
              policy: ""
            query-interval: ""
            query-last-member-interval: ""
            query-response-interval: ""
            query-source-address: ""
            router-alert-check: "true"
            sub-hosts-only: "true"
            subnet-check: "true"
            version: "3"
          group-if-query-source-address: ""
          group-interface:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            group-interface-name: ""
            import-policy: ""
            maximum-number-group-sources: ""
            maximum-number-groups: ""
            maximum-number-sources: ""
            mcac:
              bandwidth:
                mandatory: ""
                total: ""
              interface-policy: ""
              policy: ""
            query-interval: ""
            query-last-member-interval: ""
            query-response-interval: ""
            query-source-address: ""
            router-alert-check: "true"
            sub-hosts-only: "true"
            subnet-check: "true"
            version: "3"
          interface:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            import-policy: ""
            ip-interface-name: ""
            maximum-number-group-sources: ""
            maximum-number-groups: ""
            maximum-number-sources: ""
            mcac:
              bandwidth:
                mandatory: ""
                total: ""
              interface-policy: ""
              mc-constraints:
                level:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  bandwidth: ""
                  level-id: ""
                number-down:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  level: ""
                  number-lag-port-down: ""
                use-lag-port-weight: "false"
              policy: ""
            query-interval: ""
            query-last-member-interval: ""
            query-response-interval: ""
            redundant-mcast: "false"
            router-alert-check: "true"
            ssm-translate:
              group-range:
              - apply-groups: ""
                apply-groups-exclude: ""
                end: ""
                source:
                - source-address: ""
                start: ""
            static:
              group:
              - apply-groups: ""
                apply-groups-exclude: ""
                group-address: ""
                source:
                - source-address: ""
                starg: ""
              group-range:
              - apply-groups: ""
                apply-groups-exclude: ""
                end: ""
                source:
                - source-address: ""
                starg: ""
                start: ""
                step: ""
            subnet-check: "true"
            version: "3"
          query-interval: "125"
          query-last-member-interval: "1"
          query-response-interval: "10"
          robust-count: "2"
          ssm-translate:
            group-range:
            - apply-groups: ""
              apply-groups-exclude: ""
              end: ""
              source:
              - source-address: ""
              start: ""
        igmp-host-tracking:
          admin-state: disable
          apply-groups: ""
          apply-groups-exclude: ""
          expiry-time: "260"
        ignore-nh-metric: "false"
        interface:
        - admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          cflowd-parameters:
            sampling:
            - apply-groups: ""
              apply-groups-exclude: ""
              direction: ingress-only
              sample-profile: ""
              sampling-type: ""
              type: ""
          cpu-protection: ""
          description: ""
          dynamic-tunnel-redundant-nexthop: ""
          hold-time:
            ipv4:
              down:
                init-only: "false"
                seconds: ""
              up:
                seconds: ""
            ipv6:
              down:
                init-only: "false"
                seconds: ""
              up:
                seconds: ""
          if-attribute:
            admin-group: ""
            srlg-group:
            - name: ""
          ingress:
            destination-class-lookup: "false"
            policy-accounting: ""
          ingress-stats: "false"
          interface-name: ""
          ip-mtu: ""
          ipv4:
            addresses:
              address:
              - apply-groups: ""
                apply-groups-exclude: ""
                ipv4-address: ""
                prefix-length: ""
            allow-directed-broadcasts: "false"
            bfd:
              admin-state: disable
              echo-receive: ""
              multiplier: "3"
              receive: "100"
              transmit-interval: "100"
              type: auto
            dhcp:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              gi-address: ""
              lease-populate:
                max-leases: "0"
              option-82:
                action: keep
                circuit-id:
                  ascii-tuple: ""
                  ifindex: ""
                  none: ""
                  sap-id: ""
                  vlan-ascii-tuple: ""
                remote-id:
                  ascii-string: ""
                  mac: ""
                  none: ""
                vendor-specific-option:
                  client-mac-address: "false"
                  pool-name: "false"
                  sap-id: "false"
                  service-id: "false"
                  string: ""
                  system-id: "false"
              proxy-server:
                admin-state: disable
                emulated-server: ""
                lease-time:
                  radius-override: "false"
                  value: ""
              python-policy: ""
              relay-plain-bootp: "false"
              relay-proxy:
                release-update-src-ip: "false"
                siaddr-override: ""
              server: ""
              src-ip-addr: auto
              trusted: "false"
              use-arp: "false"
            icmp:
              mask-reply: "true"
              param-problem:
                admin-state: enable
                number: "100"
                seconds: "10"
              redirects:
                admin-state: enable
                number: "100"
                seconds: "10"
              ttl-expired:
                admin-state: enable
                number: "100"
                seconds: "10"
              unreachables:
                admin-state: enable
                number: "100"
                seconds: "10"
            ip-helper-address: ""
            local-dhcp-server: ""
            neighbor-discovery:
              host-route:
                populate:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  route-tag: ""
                  route-type: ""
              learn-unsolicited: "false"
              limit:
                log-only: "false"
                max-entries: ""
                threshold: "90"
              local-proxy-arp: "false"
              populate: "false"
              populate-host: "false"
              proactive-refresh: "false"
              proxy-arp-policy: ""
              remote-proxy-arp: "false"
              retry-timer: "50"
              route-tag: ""
              static-neighbor:
              - apply-groups: ""
                apply-groups-exclude: ""
                ipv4-address: ""
                mac-address: ""
              static-neighbor-unnumbered:
                mac-address: ""
              timeout: "14400"
            primary:
              address: ""
              apply-groups: ""
              apply-groups-exclude: ""
              broadcast: host-ones
              prefix-length: ""
              track-srrp: ""
            qos-route-lookup: ""
            secondary:
            - address: ""
              apply-groups: ""
              apply-groups-exclude: ""
              broadcast: host-ones
              igp-inhibit: "false"
              prefix-length: ""
              track-srrp: ""
            tcp-mss: ""
            unnumbered:
              ip-address: ""
              ip-int-name: ""
            urpf-check:
              ignore-default: "false"
              mode: strict
            vrrp:
            - admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              authentication-key: ""
              backup: ""
              bfd-liveness:
                apply-groups: ""
                apply-groups-exclude: ""
                dest-ip: ""
                interface-name: ""
                service-name: ""
              init-delay: ""
              mac: ""
              master-int-inherit: "false"
              message-interval: "10"
              ntp-reply: "false"
              oper-group: ""
              owner: "false"
              passive: "false"
              ping-reply: "false"
              policy: ""
              preempt: "true"
              priority: ""
              ssh-reply: "false"
              standby-forwarding: "false"
              telnet-reply: "false"
              traceroute-reply: "false"
              virtual-router-id: ""
          ipv6:
            address:
            - apply-groups: ""
              apply-groups-exclude: ""
              duplicate-address-detection: "true"
              eui-64: "false"
              ipv6-address: ""
              prefix-length: ""
              primary-preference: ""
              track-srrp: ""
            bfd:
              admin-state: disable
              echo-receive: ""
              multiplier: "3"
              receive: "100"
              transmit-interval: "100"
              type: auto
            dhcp6:
              apply-groups: ""
              apply-groups-exclude: ""
              relay:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                description: ""
                lease-populate:
                  max-nbr-of-leases: "0"
                  route-populate:
                    na: "false"
                    pd:
                      exclude: "false"
                    ta: "false"
                link-address: ""
                neighbor-resolution: "false"
                option:
                  apply-groups: ""
                  apply-groups-exclude: ""
                  interface-id:
                    ascii-tuple: ""
                    if-index: ""
                    sap-id: ""
                    string: ""
                  remote-id: "false"
                python-policy: ""
                server: ""
                source-address: ""
                user-db: ""
              server:
                apply-groups: ""
                apply-groups-exclude: ""
                max-nbr-of-leases: "8000"
                prefix-delegation:
                  admin-state: disable
                  prefix:
                  - apply-groups: ""
                    apply-groups-exclude: ""
                    client-id:
                      duid: ""
                      iaid: ""
                    ipv6-prefix: ""
                    preferred-lifetime: "604800"
                    valid-lifetime: "2592000"
            duplicate-address-detection: "true"
            forward-ipv4-packets: "false"
            icmp6:
              packet-too-big:
                admin-state: enable
                number: "100"
                seconds: "10"
              param-problem:
                admin-state: enable
                number: "100"
                seconds: "10"
              redirects:
                admin-state: enable
                number: "100"
                seconds: "10"
              time-exceeded:
                admin-state: enable
                number: "100"
                seconds: "10"
              unreachables:
                admin-state: enable
                number: "100"
                seconds: "10"
            link-local-address:
              address: ""
              duplicate-address-detection: "true"
            local-dhcp-server: ""
            neighbor-discovery:
              host-route:
                populate:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  route-tag: ""
                  route-type: ""
              learn-unsolicited: ""
              limit:
                log-only: "false"
                max-entries: ""
                threshold: "90"
              local-proxy-nd: "false"
              populate-host: "false"
              proactive-refresh: ""
              proxy-nd-policy: ""
              reachable-time: ""
              route-tag: ""
              secure-nd:
                admin-state: disable
                allow-unsecured-msgs: "true"
                public-key-min-bits: "1024"
                security-parameter: "1"
              stale-time: ""
              static-neighbor:
              - apply-groups: ""
                apply-groups-exclude: ""
                ipv6-address: ""
                mac-address: ""
            qos-route-lookup: ""
            tcp-mss: ""
            urpf-check:
              ignore-default: "false"
              mode: strict
            vrrp:
            - admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              backup: ""
              bfd-liveness:
                apply-groups: ""
                apply-groups-exclude: ""
                dest-ip: ""
                interface-name: ""
                service-name: ""
              init-delay: ""
              mac: ""
              master-int-inherit: "true"
              message-interval: "100"
              ntp-reply: "false"
              oper-group: ""
              owner: "false"
              passive: "false"
              ping-reply: "false"
              policy: ""
              preempt: "true"
              priority: ""
              standby-forwarding: "false"
              telnet-reply: "false"
              traceroute-reply: "false"
              virtual-router-id: ""
          load-balancing:
            ip-load-balancing: both
            spi-load-balancing: "false"
            teid-load-balancing: "false"
          loopback: "false"
          mac: ""
          mac-accounting: "false"
          monitor-oper-group: ""
          ping-template:
            admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            destination-address: ""
            name: ""
          radius-auth-policy: ""
          sap:
          - accounting-policy: ""
            admin-state: enable
            anti-spoof: ""
            apply-groups: ""
            apply-groups-exclude: ""
            bandwidth: ""
            calling-station-id: ""
            collect-stats: "false"
            cpu-protection:
              eth-cfm-monitoring:
                aggregate: ""
                car: ""
              ip-src-monitoring: ""
              mac-monitoring: ""
              policy-id: ""
            description: ""
            dist-cpu-protection: ""
            egress:
              agg-rate:
                cir: "0"
                limit-unused-bandwidth: "false"
                queue-frame-based-accounting: "false"
                rate: ""
              filter:
                ip: ""
                ipv6: ""
              qos:
                egress-remark-policy:
                  policy-name: ""
                policer-control-policy:
                  overrides:
                    apply-groups: ""
                    apply-groups-exclude: ""
                    root:
                      max-rate: ""
                      priority-mbs-thresholds:
                        min-thresh-separation: ""
                        priority:
                        - apply-groups: ""
                          apply-groups-exclude: ""
                          mbs-contribution: ""
                          priority-level: ""
                  policy-name: ""
                qinq-mark-top-only: "false"
                sap-egress:
                  overrides:
                    hs-secondary-shaper: ""
                    hs-wrr-group:
                    - apply-groups: ""
                      apply-groups-exclude: ""
                      group-id: ""
                      hs-class-weight: ""
                      percent-rate: ""
                      rate: ""
                    hsmda-queues:
                      packet-byte-offset: ""
                      queue:
                      - apply-groups: ""
                        apply-groups-exclude: ""
                        mbs: ""
                        queue-id: ""
                        rate: ""
                        slope-policy: ""
                        wrr-weight: ""
                      secondary-shaper: ""
                      wrr-policy: ""
                    policer:
                    - apply-groups: ""
                      apply-groups-exclude: ""
                      cbs: ""
                      mbs: ""
                      packet-byte-offset: ""
                      policer-id: ""
                      stat-mode: ""
                    queue:
                    - adaptation-rule:
                        cir: ""
                        pir: ""
                      apply-groups: ""
                      apply-groups-exclude: ""
                      avg-frame-overhead: ""
                      burst-limit: ""
                      cbs: ""
                      drop-tail:
                        low:
                          percent-reduction-from-mbs: ""
                      hs-class-weight: ""
                      hs-wred-queue:
                        policy: ""
                      hs-wrr-weight: "1"
                      mbs: ""
                      monitor-depth: "false"
                      monitor-queue-depth:
                        fast-polling: "false"
                        violation-threshold: ""
                      parent:
                        cir-weight: ""
                        weight: ""
                      queue-id: ""
                  policy-name: ""
                  port-redirect-group:
                    group-name: ""
                    instance: ""
                scheduler-policy:
                  overrides:
                    scheduler:
                    - apply-groups: ""
                      apply-groups-exclude: ""
                      parent:
                        cir-weight: ""
                        weight: ""
                      rate:
                        cir: ""
                        pir: ""
                      scheduler-name: ""
                  policy-name: ""
                vlan-qos-policy:
                  policy-name: ""
              queue-group-redirect-list: ""
            eth-cfm:
              apply-groups: ""
              apply-groups-exclude: ""
              collect-lmm-fc-stats:
                fc: ""
                fc-in-profile: ""
              collect-lmm-stats: "false"
              mep:
              - admin-state: disable
                ais: "false"
                alarm-notification:
                  fng-alarm-time: ""
                  fng-reset-time: ""
                apply-groups: ""
                apply-groups-exclude: ""
                ccm: "false"
                ccm-ltm-priority: "7"
                ccm-padding-size: ""
                csf:
                  multiplier: "3.5"
                description: ""
                eth-test:
                  bit-error-threshold: "1"
                  test-pattern:
                    crc-tlv: "false"
                    pattern: all-zeros
                fault-propagation: ""
                grace:
                  eth-ed:
                    max-rx-defect-window: ""
                    priority: ""
                    rx-eth-ed: "true"
                    tx-eth-ed: "false"
                  eth-vsm-grace:
                    rx-eth-vsm-grace: "true"
                    tx-eth-vsm-grace: "true"
                low-priority-defect: mac-rem-err-xcon
                ma-admin-name: ""
                md-admin-name: ""
                mep-id: ""
                one-way-delay-threshold: "3"
              squelch-ingress-levels: ""
            fwd-wholesale:
              pppoe-service: ""
            host-admin-state: enable
            host-lockout-policy: ""
            ingress:
              aggregate-policer:
                burst: default
                cbs: ""
                cir: ""
                rate: max
              filter:
                ip: ""
                ipv6: ""
              qos:
                match-qinq-dot1p: ""
                policer-control-policy:
                  overrides:
                    apply-groups: ""
                    apply-groups-exclude: ""
                    root:
                      max-rate: ""
                      priority-mbs-thresholds:
                        min-thresh-separation: ""
                        priority:
                        - apply-groups: ""
                          apply-groups-exclude: ""
                          mbs-contribution: ""
                          priority-level: ""
                  policy-name: ""
                sap-ingress:
                  fp-redirect-group:
                    group-name: ""
                    instance: ""
                  overrides:
                    ip-criteria:
                      activate-entry-tag: ""
                    ipv6-criteria:
                      activate-entry-tag: ""
                    policer:
                    - apply-groups: ""
                      apply-groups-exclude: ""
                      cbs: ""
                      mbs: ""
                      packet-byte-offset: ""
                      policer-id: ""
                      stat-mode: ""
                    queue:
                    - adaptation-rule:
                        cir: ""
                        pir: ""
                      apply-groups: ""
                      apply-groups-exclude: ""
                      cbs: ""
                      drop-tail:
                        low:
                          percent-reduction-from-mbs: ""
                      mbs: ""
                      monitor-depth: "false"
                      parent:
                        cir-weight: ""
                        weight: ""
                      queue-id: ""
                  policy-name: ""
                  queuing-type: ""
                scheduler-policy:
                  overrides:
                    scheduler:
                    - apply-groups: ""
                      apply-groups-exclude: ""
                      parent:
                        cir-weight: ""
                        weight: ""
                      rate:
                        cir: ""
                        pir: ""
                      scheduler-name: ""
                  policy-name: ""
              queue-group-redirect-list: ""
            ip-tunnel:
            - admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              backup-remote-ip-address: ""
              clear-df-bit: "false"
              delivery-service: ""
              description: ""
              dest-ip:
              - dest-ip-address: ""
              dscp: ""
              encapsulated-ip-mtu: ""
              gre-header:
                admin-state: ""
                key:
                  admin-state: disable
                  receive: "0"
                  send: "0"
              icmp6-generation:
                packet-too-big:
                  admin-state: ""
                  number: "100"
                  seconds: "10"
              ip-mtu: ""
              local-ip-address: ""
              private-tcp-mss-adjust: ""
              public-tcp-mss-adjust: ""
              reassembly: ""
              remote-ip-address: ""
              tunnel-name: ""
            ipsec-gateway:
            - admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              cert:
                cert-profile: ""
                status-verify:
                  default-result: revoked
                  primary: crl
                  secondary: none
                trust-anchor-profile: ""
              client-db:
                fallback: "true"
                name: ""
              default-secure-service:
                interface: ""
                service-name: ""
              default-tunnel-template: ""
              dhcp-address-assignment:
                dhcpv4:
                  admin-state: disable
                  apply-groups: ""
                  apply-groups-exclude: ""
                  gi-address: ""
                  send-release: "true"
                  server:
                    address: ""
                    router-instance: ""
                dhcpv6:
                  admin-state: disable
                  apply-groups: ""
                  apply-groups-exclude: ""
                  link-address: ""
                  send-release: "true"
                  server:
                    address: ""
                    router-instance: ""
              ike-policy: ""
              local:
                address-assignment:
                  admin-state: disable
                  apply-groups: ""
                  apply-groups-exclude: ""
                  ipv4:
                    dhcp-server: ""
                    pool: ""
                    router-instance: ""
                    secondary-pool: ""
                  ipv6:
                    dhcp-server: ""
                    pool: ""
                    router-instance: ""
                gateway-address: ""
                id:
                  auto: ""
                  fqdn: ""
                  ipv4: ""
                  ipv6: ""
              max-history-key-records:
                esp: ""
                ike: ""
              name: ""
              pre-shared-key: ""
              radius:
                accounting-policy: ""
                authentication-policy: ""
              ts-list: ""
            ipsec-tunnel:
            - admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              bfd:
                bfd-designate: "false"
                bfd-liveness:
                  dest-ip: ""
                  interface: ""
                  service-name: ""
              clear-df-bit: "false"
              description: ""
              dest-ip:
              - address: ""
              encapsulated-ip-mtu: ""
              icmp6-generation:
                packet-too-big:
                  admin-state: enable
                  interval: "10"
                  message-count: "100"
              ip-mtu: ""
              key-exchange:
                dynamic:
                  auto-establish: "false"
                  cert:
                    cert-profile: ""
                    status-verify:
                      default-result: revoked
                      primary: crl
                      secondary: none
                    trust-anchor-profile: ""
                  id:
                    fqdn: ""
                    ipv4: ""
                    ipv6: ""
                  ike-policy: ""
                  ipsec-transform: ""
                  pre-shared-key: ""
                manual:
                  keys:
                  - apply-groups: ""
                    apply-groups-exclude: ""
                    authentication-key: ""
                    direction: ""
                    encryption-key: ""
                    ipsec-transform: ""
                    security-association: ""
                    spi: ""
              max-history-key-records:
                esp: ""
                ike: ""
              name: ""
              private-tcp-mss-adjust: ""
              public-tcp-mss-adjust: ""
              replay-window: ""
              security-policy:
                id: ""
                strict-match: "false"
              tunnel-endpoint:
                delivery-service: ""
                local-gateway-address: ""
                remote-ip-address: ""
            lag:
              link-map-profile: ""
              per-link-hash:
                class: "1"
                weight: "1"
            multi-service-site: ""
            sap-id: ""
            static-host:
              ipv4:
              - admin-state: disable
                ancp-string: ""
                apply-groups: ""
                apply-groups-exclude: ""
                int-dest-id: ""
                ip: ""
                mac: ""
                sla-profile: ""
                sub-profile: ""
                subscriber-id:
                  string: ""
                  use-sap-id: ""
          shcv-policy-ipv4: ""
          spoke-sdp:
          - accounting-policy: ""
            admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            bfd:
              bfd-liveness:
                encap: ipv4
              bfd-template: ""
              failure-action: none
              wait-for-up-timer: ""
            bfd-liveness:
              encap: ipv4
            bfd-template: ""
            collect-stats: "false"
            control-word: "false"
            cpu-protection:
              eth-cfm-monitoring:
                aggregate: ""
                car: ""
              ip-src-monitoring: ""
              mac-monitoring: ""
              policy-id: ""
            description: ""
            egress:
              filter:
                ip: ""
                ipv6: ""
              qos:
                network:
                  policy-name: ""
                  port-redirect-group:
                    group-name: ""
                    instance: ""
              vc-label: ""
            entropy: ""
            eth-cfm:
              apply-groups: ""
              apply-groups-exclude: ""
              collect-lmm-fc-stats:
                fc: ""
                fc-in-profile: ""
              collect-lmm-stats: "false"
              mep:
              - admin-state: disable
                ais: "false"
                alarm-notification:
                  fng-alarm-time: ""
                  fng-reset-time: ""
                apply-groups: ""
                apply-groups-exclude: ""
                ccm: "false"
                ccm-ltm-priority: "7"
                ccm-padding-size: ""
                csf:
                  multiplier: "3.5"
                description: ""
                eth-test:
                  bit-error-threshold: "1"
                  test-pattern:
                    crc-tlv: "false"
                    pattern: all-zeros
                fault-propagation: ""
                grace:
                  eth-ed:
                    max-rx-defect-window: ""
                    priority: ""
                    rx-eth-ed: "true"
                    tx-eth-ed: "false"
                  eth-vsm-grace:
                    rx-eth-vsm-grace: "true"
                    tx-eth-vsm-grace: "true"
                low-priority-defect: mac-rem-err-xcon
                ma-admin-name: ""
                md-admin-name: ""
                mep-id: ""
                one-way-delay-threshold: "3"
              squelch-ingress-levels: ""
            ingress:
              filter:
                ip: ""
                ipv6: ""
              qos:
                network:
                  fp-redirect-group:
                    group-name: ""
                    instance: ""
                  policy-name: ""
              vc-label: ""
            sdp-bind-id: ""
            vc-type: ether
          static-tunnel-redundant-nexthop: ""
          tos-marking-state: trusted
          tunnel: "false"
          vas-if-type: ""
          vpls:
          - apply-groups: ""
            apply-groups-exclude: ""
            egress:
              reclassify-using-qos: ""
              routed-override-filter:
                ip: ""
                ipv6: ""
            evpn:
              arp:
                advertise:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  route-tag: ""
                  route-type: ""
                flood-garp-and-unknown-req: "true"
                learn-dynamic: "true"
              nd:
                advertise:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  route-tag: ""
                  route-type: ""
                learn-dynamic: "true"
            evpn-tunnel:
              ipv6-gateway-address: ip
              supplementary-broadcast-domain: "false"
            ingress:
              routed-override-filter:
                ip: ""
                ipv6: ""
            vpls-name: ""
        ip-mirror-interface:
        - admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          description: ""
          interface-name: ""
          spoke-sdp:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            ingress:
              filter:
                ip: ""
              vc-label: ""
            sdp-bind-id: ""
        ipsec:
          allow-reverse-route-override: "false"
          allow-reverse-route-override-type: ""
          security-policy:
          - apply-groups: ""
            apply-groups-exclude: ""
            entry:
            - apply-groups: ""
              apply-groups-exclude: ""
              entry-id: ""
              local-ip:
                address: ""
                any: "false"
              local-ipv6:
                address: ""
                any: "false"
              remote-ip:
                address: ""
                any: "false"
              remote-ipv6:
                address: ""
                any: "false"
            id: ""
        ipv6:
          neighbor-discovery:
            reachable-time: "30"
            stale-time: "14400"
          router-advertisement:
            apply-groups: ""
            apply-groups-exclude: ""
            dns-options:
              apply-groups: ""
              apply-groups-exclude: ""
              rdnss-lifetime: infinite
              server: ""
            interface:
            - admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              current-hop-limit: "64"
              dns-options:
                apply-groups: ""
                apply-groups-exclude: ""
                include-rdnss: "true"
                rdnss-lifetime: ""
                server: ""
              ip-int-name: ""
              managed-configuration: "false"
              max-advertisement-interval: "600"
              min-advertisement-interval: "200"
              mtu: ""
              other-stateful-configuration: "false"
              prefix:
              - apply-groups: ""
                apply-groups-exclude: ""
                autonomous: "true"
                ipv6-prefix: ""
                on-link: "true"
                preferred-lifetime: "604800"
                valid-lifetime: "2592000"
              reachable-time: "0"
              retransmit-time: "0"
              router-lifetime: "1800"
              use-virtual-mac: "false"
        isis:
        - admin-state: disable
          advertise-passive-only: "false"
          advertise-router-capability: ""
          all-l1isis: 01:80:C2:00:00:14
          all-l2isis: 01:80:C2:00:00:15
          apply-groups: ""
          apply-groups-exclude: ""
          area-address: ""
          authentication-check: "true"
          authentication-key: ""
          authentication-keychain: ""
          authentication-type: ""
          csnp-authentication: "true"
          default-route-tag: ""
          export-limit:
            log-percent: ""
            number: ""
          export-policy: ""
          graceful-restart:
            helper-mode: "true"
          hello-authentication: "true"
          hello-padding: ""
          ignore-attached-bit: "false"
          ignore-lsp-errors: "false"
          ignore-narrow-metric: "false"
          iid-tlv: "false"
          import-policy: ""
          interface:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            bfd-liveness:
              ipv4:
                include-bfd-tlv: "false"
              ipv6:
                include-bfd-tlv: "false"
            csnp-interval: "10"
            default-instance: "false"
            hello-authentication: "true"
            hello-authentication-key: ""
            hello-authentication-keychain: ""
            hello-authentication-type: ""
            hello-padding: ""
            interface-name: ""
            interface-type: ""
            ipv4-multicast: "true"
            ipv6-unicast: "true"
            level:
            - apply-groups: ""
              apply-groups-exclude: ""
              hello-authentication-key: ""
              hello-authentication-keychain: ""
              hello-authentication-type: ""
              hello-interval: "9"
              hello-multiplier: "3"
              hello-padding: ""
              ipv4-multicast-metric: ""
              ipv6-unicast-metric: ""
              level-number: ""
              metric: ""
              passive: "false"
              priority: "64"
              sd-offset: ""
              sf-offset: ""
            level-capability: ""
            load-balancing-weight: ""
            loopfree-alternate:
              exclude: "false"
              policy-map:
                route-nh-template: ""
            lsp-pacing-interval: "100"
            mesh-group:
              blocked: ""
              value: ""
            passive: "false"
            retransmit-interval: "5"
            tag: ""
          ipv4-multicast-routing: native
          ipv4-routing: "true"
          ipv6-routing: "false"
          isis-instance: ""
          level:
          - advertise-router-capability: "true"
            apply-groups: ""
            apply-groups-exclude: ""
            authentication-key: ""
            authentication-keychain: ""
            authentication-type: ""
            csnp-authentication: "true"
            default-ipv4-multicast-metric: "10"
            default-ipv6-unicast-metric: "10"
            default-metric: "10"
            external-preference: ""
            hello-authentication: "true"
            hello-padding: ""
            level-number: ""
            loopfree-alternate-exclude: "false"
            lsp-mtu-size: "1492"
            preference: ""
            psnp-authentication: "true"
            wide-metrics-only: "false"
          level-capability: ""
          link-group:
          - apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            level:
            - apply-groups: ""
              apply-groups-exclude: ""
              ipv4-multicast-metric-offset: ""
              ipv4-unicast-metric-offset: ""
              ipv6-unicast-metric-offset: ""
              level-number: ""
              member:
              - interface-name: ""
              oper-members: ""
              revert-members: ""
            link-group-name: ""
          loopfree-alternate:
            exclude:
              prefix-policy: ""
          lsp-lifetime: "1200"
          lsp-minimum-remaining-lifetime: ""
          lsp-mtu-size: "1492"
          lsp-refresh:
            half-lifetime: "true"
            interval: "600"
          multi-topology:
            ipv4-multicast: "false"
            ipv6-unicast: "false"
          multicast-import:
            ipv4: "false"
          overload:
            max-metric: "false"
          overload-export-external: "false"
          overload-export-interlevel: "false"
          overload-on-boot:
            max-metric: "false"
            timeout: ""
          poi-tlv: "false"
          prefix-attributes-tlv: "false"
          prefix-limit:
            limit: ""
            log-only: "false"
            overload-timeout: forever
            warning-threshold: "0"
          psnp-authentication: "true"
          reference-bandwidth: ""
          rib-priority:
            high:
              prefix-list: ""
              tag: ""
          router-id: ""
          standard-multi-instance: "false"
          strict-adjacency-check: "false"
          summary-address:
          - apply-groups: ""
            apply-groups-exclude: ""
            ip-prefix: ""
            level-capability: ""
            route-tag: ""
          suppress-attached-bit: "false"
          system-id: ""
          timers:
            lsp-wait:
              lsp-initial-wait: "10"
              lsp-max-wait: "5000"
              lsp-second-wait: "1000"
            spf-wait:
              spf-initial-wait: "1000"
              spf-max-wait: "10000"
              spf-second-wait: "1000"
          unicast-import:
            ipv4: "true"
            ipv6: "true"
        l2tp:
          admin-state: disable
          apply-groups: ""
          apply-groups-exclude: ""
          avp-hiding: ""
          challenge: "false"
          destruct-timeout: ""
          ethernet-tunnel:
            reconnect-timeout: ""
          exclude-avps:
            calling-number: "false"
            initial-rx-lcp-conf-req: "false"
          failover:
            recovery-max-session-lifetime: "2"
            recovery-method: ""
            recovery-time: ""
            track-srrp:
            - apply-groups: ""
              apply-groups-exclude: ""
              id: ""
              peer: ""
              sync-tag: ""
          group:
          - admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            avp-hiding: ""
            challenge: ""
            description: ""
            destruct-timeout: ""
            ethernet-tunnel:
              reconnect-timeout: ""
            failover:
              recovery-method: ""
              recovery-time: ""
            hello-interval: ""
            idle-timeout: ""
            l2tpv3:
              cookie-length: ""
              digest-type: ""
              nonce-length: ""
              password: ""
              private-tcp-mss-adjust: ""
              public-tcp-mss-adjust: ""
              pw-cap-list:
                ethernet: "false"
                ethernet-vlan: "false"
              rem-router-id: 0.0.0.0
              track-password-change: "false"
            lac:
              df-bit: ""
            lns:
              lns-group: ""
              load-balance-method: per-session
              mlppp:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                endpoint:
                  ip: ""
                  mac: ""
                interleave: "false"
                max-fragment-delay: no-fragmentation
                max-links: "1"
                reassembly-timeout: "1000"
                short-sequence-numbers: "false"
              ppp:
                authentication: pref-chap
                authentication-policy: ""
                chap-challenge-length:
                  end: "64"
                  start: "32"
                default-group-interface:
                  interface: ""
                  service-name: ""
                ipcp-subnet-negotiation: "false"
                keepalive:
                  interval: "30"
                  multiplier: "3"
                lcp-force-ack-accm: "false"
                lcp-ignore-magic-numbers: "false"
                mtu: "1500"
                proxy-authentication: "false"
                proxy-lcp: "false"
                reject-disabled-ncp: "false"
                user-db: ""
            local-address: ""
            local-name: ""
            max-retries-estab: ""
            max-retries-not-estab: ""
            password: ""
            protocol: ""
            radius-accounting-policy: ""
            receive-window-size: ""
            session-assign-method: ""
            session-limit: ""
            tunnel:
            - admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              auto-establish: "false"
              avp-hiding: ""
              challenge: ""
              description: ""
              destruct-timeout: ""
              failover:
                recovery-method: ""
                recovery-time: ""
              hello-interval: ""
              idle-timeout: ""
              l2tpv3:
                private-tcp-mss-adjust: ""
                public-tcp-mss-adjust: ""
              lac:
                df-bit: ""
              lns:
                lns-group: ""
                load-balance-method: ""
                mlppp:
                  admin-state: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  endpoint:
                    ip: ""
                    mac: ""
                  interleave: ""
                  max-fragment-delay: ""
                  max-links: ""
                  reassembly-timeout: ""
                  short-sequence-numbers: ""
                ppp:
                  authentication: ""
                  authentication-policy: ""
                  chap-challenge-length:
                    end: ""
                    start: ""
                  default-group-interface:
                    interface: ""
                    service-name: ""
                  ipcp-subnet-negotiation: ""
                  keepalive:
                    interval: ""
                    multiplier: ""
                  lcp-force-ack-accm: ""
                  lcp-ignore-magic-numbers: ""
                  mtu: ""
                  proxy-authentication: ""
                  proxy-lcp: ""
                  reject-disabled-ncp: ""
                  user-db: ""
              local-address: ""
              local-name: ""
              max-retries-estab: ""
              max-retries-not-estab: ""
              password: ""
              peer: ""
              preference: "50"
              radius-accounting-policy: ""
              receive-window-size: ""
              remote-name: ""
              session-limit: ""
              tunnel-name: ""
            tunnel-group-name: ""
          group-session-limit: ""
          hello-interval: ""
          idle-timeout: ""
          ignore-avps:
            sequencing-required: "false"
          l2tpv3:
            cookie-length: ""
            digest-type: ""
            nonce-length: ""
            password: ""
            private-tcp-mss-adjust: ""
            public-tcp-mss-adjust: ""
            transport-type:
              ip: "false"
          lac:
            calling-number-format: '%S %s'
            cisco-nas-port:
              ethernet: ""
            df-bit: "true"
          local-address: ""
          local-name: ""
          max-retries-estab: ""
          max-retries-not-estab: ""
          next-attempt: next-preference-level
          password: ""
          peer-address-change-policy: ""
          radius-accounting-policy: ""
          receive-window-size: ""
          replace-result-code:
            cdn-invalid-dst: "false"
            cdn-permanent-no-facilities: "false"
            cdn-temporary-no-facilities: "false"
          rtm-debounce-time: ""
          session-assign-method: ""
          session-limit: ""
          tunnel-selection-blacklist:
            add-tunnel-on:
              address-change-timeout: "false"
              cdn-err-code: "false"
              cdn-invalid-dst: "false"
              cdn-permanent-no-facilities: "false"
              cdn-temporary-no-facilities: "false"
              stop-ccn-err-code: "false"
              stop-ccn-other: "false"
              tx-cdn-not-established-in-time: "false"
            max-list-length: infinite
            max-time: "5"
            timeout-action: remove-from-blacklist
          tunnel-session-limit: ""
        label-mode: vrf
        log:
          apply-groups: ""
          apply-groups-exclude: ""
          filter:
          - apply-groups: ""
            apply-groups-exclude: ""
            default-action: forward
            description: ""
            entry:
            - action: ""
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              entry-id: ""
              match:
                application:
                  eq: ""
                  neq: ""
                event:
                  eq: ""
                  gt: ""
                  gte: ""
                  lt: ""
                  lte: ""
                  neq: ""
                message:
                  eq: ""
                  neq: ""
                  regexp: "false"
                severity:
                  eq: ""
                  gt: ""
                  gte: ""
                  lt: ""
                  lte: ""
                  neq: ""
                subject:
                  eq: ""
                  neq: ""
                  regexp: "false"
            filter-name: ""
            named-entry:
            - action: ""
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              entry-name: ""
              match:
                application:
                  eq: ""
                  neq: ""
                event:
                  eq: ""
                  gt: ""
                  gte: ""
                  lt: ""
                  lte: ""
                  neq: ""
                message:
                  eq: ""
                  neq: ""
                  regexp: "false"
                severity:
                  eq: ""
                  gt: ""
                  gte: ""
                  lt: ""
                  lte: ""
                  neq: ""
                subject:
                  eq: ""
                  neq: ""
                  regexp: "false"
          log-id:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            destination:
              netconf:
                max-entries: "100"
              snmp:
                max-entries: "100"
              syslog: ""
            filter: ""
            name: ""
            netconf-stream: ""
            python-policy: ""
            source:
              change: "false"
              debug: "false"
              main: "false"
              security: "false"
            time-format: utc
          snmp-trap-group:
          - apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            log-id: ""
            log-name: ""
            trap-target:
            - address: ""
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              name: ""
              notify-community: ""
              port: "162"
              replay: "false"
              security-level: no-auth-no-privacy
              version: snmpv3
          syslog:
          - address: ""
            apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            facility: local7
            log-prefix: TMNX
            port: "514"
            severity: info
            syslog-name: ""
        management:
          allow-ftp: "false"
          allow-grpc: "false"
          allow-netconf: "false"
          allow-ssh: "false"
          allow-telnet: "false"
          allow-telnet6: "false"
          apply-groups: ""
          apply-groups-exclude: ""
        maximum-ipv4-routes:
          log-only: "false"
          threshold: ""
          value: ""
        maximum-ipv6-routes:
          log-only: "false"
          threshold: ""
          value: ""
        mc-maximum-routes:
          log-only: "false"
          threshold: ""
          value: ""
        mld:
          admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          forwarding-group-interface:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            forwarding-service: ""
            group-interface-name: ""
            import-policy: ""
            maximum-number-group-sources: ""
            maximum-number-groups: ""
            maximum-number-sources: ""
            mcac:
              bandwidth:
                mandatory: ""
                total: ""
              interface-policy: ""
              policy: ""
            query-interval: ""
            query-last-member-interval: ""
            query-response-interval: ""
            query-source-address: ""
            router-alert-check: "true"
            sub-hosts-only: "true"
            subnet-check: "true"
            version: "2"
          group-if-query-source-address: ""
          group-interface:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            group-interface-name: ""
            import-policy: ""
            maximum-number-group-sources: ""
            maximum-number-groups: ""
            maximum-number-sources: ""
            mcac:
              bandwidth:
                mandatory: ""
                total: ""
              interface-policy: ""
              policy: ""
            query-interval: ""
            query-last-member-interval: ""
            query-response-interval: ""
            query-source-address: ""
            router-alert-check: "true"
            sub-hosts-only: "true"
            subnet-check: "true"
            version: "2"
          interface:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            import-policy: ""
            ip-interface-name: ""
            maximum-number-group-sources: ""
            maximum-number-groups: ""
            maximum-number-sources: ""
            mcac:
              bandwidth:
                mandatory: ""
                total: ""
              interface-policy: ""
              mc-constraints:
                level:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  bandwidth: ""
                  level-id: ""
                number-down:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  level: ""
                  number-lag-port-down: ""
                use-lag-port-weight: "false"
              policy: ""
            query-interval: ""
            query-last-member-interval: ""
            query-response-interval: ""
            router-alert-check: "true"
            ssm-translate:
              group-range:
              - apply-groups: ""
                apply-groups-exclude: ""
                end: ""
                source:
                - source-address: ""
                start: ""
            static:
              group:
              - apply-groups: ""
                apply-groups-exclude: ""
                group-address: ""
                source:
                - source-address: ""
                starg: ""
              group-range:
              - apply-groups: ""
                apply-groups-exclude: ""
                end: ""
                source:
                - source-address: ""
                starg: ""
                start: ""
                step: ""
            version: "2"
          query-interval: "125"
          query-last-member-interval: "1"
          query-response-interval: "10"
          robust-count: "2"
          ssm-translate:
            group-range:
            - apply-groups: ""
              apply-groups-exclude: ""
              end: ""
              source:
              - source-address: ""
              start: ""
        msdp:
          active-source-limit: ""
          admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          data-encapsulation: "true"
          export-policy: ""
          group:
          - active-source-limit: ""
            admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            export-policy: ""
            import-policy: ""
            local-address: ""
            mode: standard
            name: ""
            peer:
            - active-source-limit: ""
              admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              authentication-key: ""
              default-peer: "false"
              export-policy: ""
              import-policy: ""
              ip-address: ""
              local-address: ""
              receive-message-rate:
                rate: ""
                threshold: ""
                time: ""
            receive-message-rate:
              rate: ""
              threshold: ""
              time: ""
          import-policy: ""
          local-address: ""
          peer:
          - active-source-limit: ""
            admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            authentication-key: ""
            default-peer: "false"
            export-policy: ""
            import-policy: ""
            ip-address: ""
            local-address: ""
            receive-message-rate:
              rate: ""
              threshold: ""
              time: ""
          receive-message-rate:
            rate: ""
            threshold: ""
            time: ""
          rpf-table: rtable-u
          source:
          - active-source-limit: ""
            apply-groups: ""
            apply-groups-exclude: ""
            ip-prefix: ""
          source-active-cache-lifetime: "90"
        mss-adjust:
          apply-groups: ""
          apply-groups-exclude: ""
          nat-group: ""
          segment-size: ""
        mtrace2:
          admin-state: disable
          udp-port: "5000"
        multicast-info-policy: ""
        mvpn:
          apply-groups: ""
          apply-groups-exclude: ""
          auto-discovery:
            source-address: 0.0.0.0
            type: ""
          c-mcast-signaling: pim
          intersite-shared:
            admin-state: enable
            kat-type5-advertisement-withdraw: "false"
            persistent-type5-advertisement: "false"
          mdt-type: sender-receiver
          provider-tunnel:
            inclusive:
              bier:
                admin-state: disable
                sub-domain: ""
              bsr: ""
              mldp:
                admin-state: disable
              pim:
                admin-state: enable
                group-address: ""
                hello-interval: "30"
                hello-multiplier: "35"
                improved-assert: "true"
                mode: ""
                three-way-hello: "false"
                tracking-support: "false"
              rsvp:
                admin-state: disable
                bfd-leaf: "false"
                bfd-root:
                  multiplier: "3"
                  transmit-interval: ""
                lsp-template: ""
              wildcard-spmsi: "false"
            selective:
              asm-mdt: "false"
              auto-discovery: "false"
              bier:
                admin-state: disable
                sub-domain: ""
              data-delay-interval: "3"
              data-threshold:
                group-prefix:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  ip-group-prefix: ""
                  pe-threshold-add: "65535"
                  pe-threshold-delete: "65535"
                  threshold: ""
              join-tlv-packing: "true"
              mldp:
                admin-state: disable
                maximum-p2mp-spmsi: "10"
              multistream-spmsi:
              - admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                group-prefix:
                - ip-group-prefix: ""
                  source-prefix: ""
                lsp-template: ""
                multistream-id: ""
              pim:
                group-prefix: ""
                mode: ""
              rsvp:
                admin-state: disable
                lsp-template: ""
                maximum-p2mp-spmsi: "10"
          redundant-source-list:
            source-prefix:
            - ip-prefix: ""
          rpf-select:
            core-mvpn:
            - apply-groups: ""
              apply-groups-exclude: ""
              core-mvpn-service-name: ""
              group-prefix:
              - apply-groups: ""
                apply-groups-exclude: ""
                ip-group-prefix: ""
                starg: "false"
          umh-pe-backup:
            umh-pe:
            - apply-groups: ""
              apply-groups-exclude: ""
              ip-address: ""
              standby: ""
          umh-selection: highest-ip
          vrf-export:
            policy: ""
            unicast: "false"
          vrf-import:
            policy: ""
            unicast: "false"
          vrf-target:
            community: ""
            export:
              community: ""
              unicast: "false"
            import:
              community: ""
              unicast: "false"
            unicast: "false"
        nat:
          apply-groups: ""
          apply-groups-exclude: ""
          inside:
            l2-aware:
              subscribers:
              - prefix: ""
            large-scale:
              dnat-only:
                source-prefix-list: ""
              dual-stack-lite:
                admin-state: disable
                deterministic:
                  policy-map:
                  - admin-state: disable
                    apply-groups: ""
                    apply-groups-exclude: ""
                    map:
                    - apply-groups: ""
                      apply-groups-exclude: ""
                      first-outside-address: ""
                      from: ""
                      to: ""
                    nat-policy: ""
                    source-prefix: ""
                endpoint:
                - address: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  ip-fragmentation: ""
                  reassembly: "false"
                  tunnel-mtu: "1500"
                max-subscriber-limit: ""
                subscriber-prefix-length: ""
              filters:
                downstream:
                  ipv4: ""
              nat-policy: ""
              nat44:
                destination-prefix:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  ip-prefix-length: ""
                  nat-policy: ""
                deterministic:
                  policy-map:
                  - admin-state: disable
                    apply-groups: ""
                    apply-groups-exclude: ""
                    map:
                    - apply-groups: ""
                      apply-groups-exclude: ""
                      first-outside-address: ""
                      from: ""
                      to: ""
                    nat-policy: ""
                    source-prefix: ""
                max-subscriber-limit: ""
              nat64:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                drop-zero-ipv4-checksum: "false"
                insert-ipv6-fragment-header: "false"
                ip-fragmentation: ""
                ipv6-mtu: "1520"
                prefix: 64:ff9b::/96
                subscriber-prefix-length: ""
                tos:
                  downstream:
                    use-ipv4: "false"
                  upstream:
                    set-tos: use-ipv6
              redundancy:
                peer: ""
                peer6: ""
                steering-route: ""
              subscriber-identification:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                attribute:
                  type: alc-sub-string
                  vendor: nokia
                description: ""
                drop-unidentified-traffic: "false"
                radius-proxy-server:
                  router-instance: ""
                  server: ""
          map:
            map-domain:
            - domain-name: ""
          outside:
            dnat-only:
              route-limit: "32768"
            filters:
              downstream:
                ipv4: ""
                ipv6: ""
              upstream:
                ipv4: ""
                ipv6: ""
            mtu: ""
            pool:
            - address-range:
              - apply-groups: ""
                apply-groups-exclude: ""
                description: ""
                drain: "false"
                end: ""
                start: ""
              admin-state: disable
              applications:
                agnostic: "false"
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              l2-aware:
                external-assignment: "false"
              large-scale:
                deterministic:
                  port-reservation: ""
                redundancy:
                  admin-state: disable
                  export-route: ""
                  follow:
                    name: ""
                    router-instance: ""
                  monitor-route: ""
                subscriber-limit: ""
              mode: ""
              name: ""
              nat: ""
              port-forwarding:
                dynamic-block-reservation: "false"
                range-end: ""
              port-reservation:
                port-blocks: ""
                ports: ""
              type: ""
              watermarks:
                high: ""
                low: ""
              wlan-gw: ""
        network:
          apply-groups: ""
          apply-groups-exclude: ""
          ingress:
            filter:
              ip: ""
              ipv6: ""
            qos:
              fp-redirect-group: ""
              instance: ""
              network-policy: ""
            urpf-check: "true"
        network-interface:
        - admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          cflowd-parameters:
            sampling:
            - apply-groups: ""
              apply-groups-exclude: ""
              direction: ingress-only
              sample-profile: ""
              sampling-type: ""
              type: ""
          cpu-protection: ""
          description: ""
          dist-cpu-protection: ""
          egress:
            filter:
              ip: ""
          hold-time:
            ipv4:
              down:
                init-only: "false"
                seconds: ""
              up:
                seconds: ""
          ingress:
            filter:
              ip: ""
          ingress-stats: "false"
          interface-name: ""
          ip-mtu: ""
          ipv4:
            allow-directed-broadcasts: "false"
            bfd:
              admin-state: disable
              echo-receive: ""
              multiplier: "3"
              receive: "100"
              transmit-interval: "100"
              type: auto
            icmp:
              mask-reply: "true"
              param-problem:
                admin-state: enable
                number: "100"
                seconds: "10"
              redirects:
                admin-state: enable
                number: "100"
                seconds: "10"
              ttl-expired:
                admin-state: enable
                number: "100"
                seconds: "10"
              unreachables:
                admin-state: enable
                number: "100"
                seconds: "10"
            neighbor-discovery:
              retry-timer: "50"
              static-neighbor:
              - apply-groups: ""
                apply-groups-exclude: ""
                ipv4-address: ""
                mac-address: ""
              timeout: "14400"
            primary:
              address: ""
              apply-groups: ""
              apply-groups-exclude: ""
              broadcast: host-ones
              prefix-length: ""
            secondary:
            - address: ""
              apply-groups: ""
              apply-groups-exclude: ""
              broadcast: host-ones
              igp-inhibit: "false"
              prefix-length: ""
            tcp-mss: ""
            urpf-check:
              ignore-default: "false"
              mode: strict
          lag:
            link-map-profile: ""
            per-link-hash:
              class: "1"
              weight: "1"
          load-balancing:
            ip-load-balancing: both
            lsr-load-balancing: ""
            spi-load-balancing: "false"
            teid-load-balancing: "false"
          loopback: ""
          mac: ""
          port-encap: ""
          qos:
            apply-groups: ""
            apply-groups-exclude: ""
            egress-instance: ""
            egress-port-redirect-group: ""
            ingress-fp-redirect-group: ""
            ingress-instance: ""
            network-policy: ""
          tos-marking-state: trusted
        ntp:
          admin-state: disable
          apply-groups: ""
          apply-groups-exclude: ""
          authenticate: "false"
          authentication-check: "true"
          authentication-key:
          - apply-groups: ""
            apply-groups-exclude: ""
            key: ""
            key-id: ""
            type: ""
          broadcast:
          - apply-groups: ""
            apply-groups-exclude: ""
            interface-name: ""
            key-id: ""
            ttl: "127"
            version: "4"
        ospf:
        - admin-state: disable
          advertise-router-capability: "false"
          apply-groups: ""
          apply-groups-exclude: ""
          area:
          - advertise-ne-profile: ""
            advertise-router-capability: "true"
            apply-groups: ""
            apply-groups-exclude: ""
            area-id: ""
            area-range:
            - advertise: "true"
              apply-groups: ""
              apply-groups-exclude: ""
              ip-prefix-mask: ""
            blackhole-aggregate: "true"
            export-policy: ""
            import-policy: ""
            interface:
            - admin-state: enable
              advertise-router-capability: "true"
              advertise-subnet: "true"
              apply-groups: ""
              apply-groups-exclude: ""
              authentication-key: ""
              authentication-keychain: ""
              authentication-type: ""
              bfd-liveness:
                remain-down-on-failure: "false"
              dead-interval: "40"
              hello-interval: "10"
              interface-name: ""
              interface-type: ""
              load-balancing-weight: ""
              loopfree-alternate:
                exclude: "false"
                policy-map:
                  route-nh-template: ""
              lsa-filter-out: none
              message-digest-key:
              - apply-groups: ""
                apply-groups-exclude: ""
                key-id: ""
                md5: ""
              metric: ""
              mtu: ""
              neighbor:
              - address: ""
              passive: "false"
              poll-interval: "120"
              priority: "1"
              retransmit-interval: "5"
              rib-priority: ""
              transit-delay: "1"
            loopfree-alternate-exclude: "false"
            nssa:
              area-range:
              - advertise: "true"
                apply-groups: ""
                apply-groups-exclude: ""
                ip-prefix-mask: ""
              originate-default-route:
                adjacency-check: "false"
                type-nssa: "false"
              redistribute-external: "true"
              summaries: "true"
            sham-link:
            - admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              authentication-key: ""
              authentication-keychain: ""
              authentication-type: ""
              dead-interval: "40"
              hello-interval: "10"
              interface: ""
              ip-address: ""
              message-digest-key:
              - apply-groups: ""
                apply-groups-exclude: ""
                key-id: ""
                md5: ""
              metric: "1"
              retransmit-interval: "5"
              transit-delay: "1"
            stub:
              default-metric: "1"
              summaries: "true"
            virtual-link:
            - admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              authentication-key: ""
              authentication-keychain: ""
              authentication-type: ""
              dead-interval: "60"
              hello-interval: "10"
              message-digest-key:
              - apply-groups: ""
                apply-groups-exclude: ""
                key-id: ""
                md5: ""
              retransmit-interval: "5"
              router-id: ""
              transit-area: ""
              transit-delay: "1"
          compatible-rfc1583: "true"
          export-limit:
            log-percent: ""
            number: ""
          export-policy: ""
          external-db-overflow:
            interval: "0"
            limit: "0"
          external-preference: "150"
          graceful-restart:
            helper-mode: "true"
            strict-lsa-checking: "true"
          ignore-dn-bit: "false"
          import-policy: ""
          loopfree-alternate:
            exclude:
              prefix-policy: ""
          multicast-import: "false"
          ospf-instance: ""
          overload: "false"
          overload-include-ext-1: "false"
          overload-include-ext-2: "false"
          overload-include-stub: "false"
          overload-on-boot:
            timeout: ""
          preference: "10"
          reference-bandwidth: "100000000"
          rib-priority:
            high:
              prefix-list: ""
          router-id: ""
          rtr-adv-lsa-limit:
            log-only: "false"
            max-lsa-count: ""
            overload-timeout: forever
            warning-threshold: "0"
          super-backbone: "false"
          suppress-dn-bit: "false"
          timers:
            incremental-spf-wait: "1000"
            lsa-accumulate: "1000"
            lsa-arrival: "1000"
            lsa-generate:
              lsa-initial-wait: "5000"
              lsa-second-wait: "5000"
              max-lsa-wait: "5000"
            redistribute-delay: "1000"
            spf-wait:
              spf-initial-wait: "1000"
              spf-max-wait: "10000"
              spf-second-wait: "1000"
          unicast-import: "true"
          vpn-domain:
            id: ""
            type: ""
          vpn-tag: "0"
        ospf3:
        - admin-state: disable
          advertise-router-capability: "false"
          apply-groups: ""
          apply-groups-exclude: ""
          area:
          - advertise-router-capability: "true"
            apply-groups: ""
            apply-groups-exclude: ""
            area-id: ""
            area-range:
            - advertise: "true"
              apply-groups: ""
              apply-groups-exclude: ""
              ip-prefix-mask: ""
            blackhole-aggregate: "true"
            export-policy: ""
            import-policy: ""
            interface:
            - admin-state: enable
              advertise-router-capability: "true"
              apply-groups: ""
              apply-groups-exclude: ""
              authentication:
                inbound: ""
                outbound: ""
              bfd-liveness:
                remain-down-on-failure: "false"
              dead-interval: "40"
              hello-interval: "10"
              interface-name: ""
              interface-type: ""
              load-balancing-weight: ""
              loopfree-alternate:
                exclude: "false"
                policy-map:
                  route-nh-template: ""
              lsa-filter-out: none
              metric: ""
              mtu: ""
              neighbor:
              - address: ""
              passive: "false"
              poll-interval: "120"
              priority: "1"
              retransmit-interval: "5"
              rib-priority: ""
              transit-delay: "1"
            key-rollover-interval: "10"
            loopfree-alternate-exclude: "false"
            nssa:
              area-range:
              - advertise: "true"
                apply-groups: ""
                apply-groups-exclude: ""
                ip-prefix-mask: ""
              originate-default-route:
                adjacency-check: "false"
                type-nssa: "false"
              redistribute-external: "true"
              summaries: "true"
            stub:
              default-metric: "1"
              summaries: "true"
            virtual-link:
            - admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              authentication:
                inbound: ""
                outbound: ""
              dead-interval: "60"
              hello-interval: "10"
              retransmit-interval: "5"
              router-id: ""
              transit-area: ""
              transit-delay: "1"
          export-limit:
            log-percent: ""
            number: ""
          export-policy: ""
          external-db-overflow:
            interval: "0"
            limit: "0"
          external-preference: "150"
          graceful-restart:
            helper-mode: "true"
            strict-lsa-checking: "true"
          ignore-dn-bit: "false"
          import-policy: ""
          loopfree-alternate:
            exclude:
              prefix-policy: ""
          multicast-import: "false"
          ospf-instance: ""
          overload: "false"
          overload-include-ext-1: "false"
          overload-include-ext-2: "false"
          overload-include-stub: "false"
          overload-on-boot:
            timeout: ""
          preference: "10"
          reference-bandwidth: "100000000"
          rib-priority:
            high:
              prefix-list: ""
          router-id: ""
          rtr-adv-lsa-limit:
            log-only: "false"
            max-lsa-count: ""
            overload-timeout: forever
            warning-threshold: "0"
          suppress-dn-bit: "false"
          timers:
            incremental-spf-wait: "1000"
            lsa-accumulate: "1000"
            lsa-arrival: "1000"
            lsa-generate:
              lsa-initial-wait: "5000"
              lsa-second-wait: "5000"
              max-lsa-wait: "5000"
            redistribute-delay: "1000"
            spf-wait:
              spf-initial-wait: "1000"
              spf-max-wait: "10000"
              spf-second-wait: "1000"
          unicast-import: "true"
        pcp:
          server:
          - admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            dual-stack-lite-address: ""
            fwd-inside-router: ""
            interface:
            - name: ""
            name: ""
            policy: ""
        pim:
          admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          apply-to: none
          bgp-nh-override: "false"
          import:
            join-policy: ""
            register-policy: ""
          interface:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            assert-period: "60"
            bfd-liveness:
              ipv4: "false"
              ipv6: "false"
            bsm-check-rtr-alert: "false"
            hello-interval: "30"
            hello-multiplier: "35"
            improved-assert: "true"
            instant-prune-echo: "false"
            interface-name: ""
            ipv4:
              apply-groups: ""
              apply-groups-exclude: ""
              monitor-oper-group:
                name: ""
                operation: ""
                priority-delta: ""
              multicast: "true"
            ipv6:
              apply-groups: ""
              apply-groups-exclude: ""
              monitor-oper-group:
                name: ""
                operation: ""
                priority-delta: ""
              multicast: "true"
            max-groups: "0"
            mcac:
              bandwidth:
                mandatory: ""
                total: ""
              interface-policy: ""
              mc-constraints:
                admin-state: enable
                level:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  bandwidth: ""
                  level-id: ""
                number-down:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  level: ""
                  number-lag-port-down: ""
                use-lag-port-weight: "false"
              policy: ""
            multicast-senders: auto
            p2mp-ldp-tree-join:
              ipv4: "false"
              ipv6: "false"
            priority: "1"
            sticky-dr:
              priority: "1024"
            three-way-hello: "false"
            tracking-support: "false"
          ipv4:
            admin-state: enable
            grt-extranet:
              any: ""
              group-prefix:
              - apply-groups: ""
                apply-groups-exclude: ""
                ip-prefix: ""
                starg: "false"
            rpf-table: rtable-u
            ssm-assert-compatible-mode: "false"
            ssm-default-range: "true"
          ipv6:
            admin-state: disable
            rpf-table: rtable-u
            ssm-default-range: "true"
          mc-ecmp-balance: "true"
          mc-ecmp-balance-hold: ""
          mc-ecmp-hashing:
            rebalance: "false"
          mtu-over-head: "0"
          non-dr-attract-traffic: "false"
          rp:
            bootstrap:
              export: ""
              import: ""
            ipv4:
              anycast:
              - ipv4-address: ""
                rp-set-peer: ""
              auto-rp-discovery: "false"
              bsr-candidate:
                address: ""
                admin-state: disable
                hash-mask-len: "30"
                priority: "0"
              candidate: "false"
              mapping-agent: "false"
              rp-candidate:
                address: ""
                admin-state: disable
                group-range:
                - ipv4-prefix: ""
                holdtime: "150"
                priority: "192"
              static:
                address:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  group-prefix:
                  - ipv4-prefix: ""
                  ipv4-address: ""
                  override: "false"
            ipv6:
              anycast:
              - ipv6-address: ""
                rp-set-peer: ""
              bsr-candidate:
                address: ""
                admin-state: disable
                hash-mask-len: "126"
                priority: "0"
              embedded-rp:
                admin-state: disable
                group-range:
                - ipv6-prefix: ""
              rp-candidate:
                address: ""
                admin-state: disable
                group-range:
                - ipv6-prefix: ""
                holdtime: "150"
                priority: "192"
              static:
                address:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  group-prefix:
                  - ipv6-prefix: ""
                  ipv6-address: ""
                  override: "false"
          spt-switchover:
          - apply-groups: ""
            apply-groups-exclude: ""
            ip-prefix: ""
            threshold: ""
          ssm-groups:
            group-range:
            - ip-prefix: ""
        radius:
          apply-groups: ""
          apply-groups-exclude: ""
          proxy:
          - admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            attribute-matching:
              entry:
              - accounting-server-policy: ""
                apply-groups: ""
                apply-groups-exclude: ""
                authentication-server-policy: ""
                index: ""
                prefix-string: ""
                suffix-string: ""
              type: ""
              vendor: ""
            cache:
              admin-state: disable
              key:
                attribute-type: ""
                packet-type: ""
                vendor: ""
              timeout: "300"
              track-accounting:
                accounting-off: "false"
                accounting-on: "false"
                interim-update: "false"
                start: "false"
                stop: "false"
              track-authentication:
                accept: "true"
              track-delete-hold-time: "0"
            defaults:
              accounting-server-policy: ""
              authentication-server-policy: ""
            description: ""
            interface:
            - interface-name: ""
            load-balance-key:
              attribute-1:
                type: ""
                vendor: ""
              attribute-2:
                type: ""
                vendor: ""
              attribute-3:
                type: ""
                vendor: ""
              attribute-4:
                type: ""
                vendor: ""
              attribute-5:
                type: ""
                vendor: ""
              source-ip-udp: ""
            name: ""
            purpose: ""
            python-policy: ""
            secret: ""
            send-accounting-response: "false"
            wlan-gw:
              address: ""
              apply-groups: ""
              apply-groups-exclude: ""
              ipv6-address: ""
            wlan-gw-group: ""
          server:
          - accept-coa: "false"
            acct-port: "1813"
            address: ""
            apply-groups: ""
            apply-groups-exclude: ""
            auth-port: "1812"
            description: ""
            name: ""
            pending-requests-limit: "4096"
            python-policy: ""
            secret: ""
        reassembly:
          apply-groups: ""
          apply-groups-exclude: ""
          nat-group: ""
          to-base-network: "false"
        redundant-interface:
        - admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          description: ""
          hold-time:
            ipv4:
              down:
                init-only: "false"
                seconds: ""
              up:
                seconds: ""
          interface-name: ""
          ip-mtu: ""
          ipv4:
            primary:
              address: ""
              apply-groups: ""
              apply-groups-exclude: ""
              prefix-length: ""
              remote-ip: ""
          spoke-sdp:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            control-word: "false"
            description: ""
            egress:
              filter:
                ip: ""
              vc-label: ""
            ingress:
              filter:
                ip: ""
              vc-label: ""
            sdp-bind-id: ""
        rip:
          admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          authentication-key: ""
          authentication-type: none
          bfd-liveness: "false"
          check-zero: "false"
          description: ""
          export-limit:
            log-percent: ""
            number: ""
          export-policy: ""
          group:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            authentication-key: ""
            authentication-type: ""
            bfd-liveness: ""
            check-zero: ""
            description: ""
            export-policy: ""
            group-name: ""
            import-policy: ""
            message-size: ""
            metric-in: ""
            metric-out: ""
            neighbor:
            - admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              authentication-key: ""
              authentication-type: ""
              bfd-liveness: ""
              check-zero: ""
              description: ""
              export-policy: ""
              import-policy: ""
              interface-name: ""
              message-size: ""
              metric-in: ""
              metric-out: ""
              preference: ""
              receive: ""
              send: ""
              split-horizon: ""
              timers:
                flush: ""
                timeout: ""
                update: ""
              unicast-address:
              - address: ""
            preference: ""
            receive: ""
            send: ""
            split-horizon: ""
            timers:
              flush: ""
              timeout: ""
              update: ""
          import-policy: ""
          message-size: "25"
          metric-in: "1"
          metric-out: "1"
          preference: "100"
          propagate-metric: "false"
          receive: both
          send: broadcast
          split-horizon: "true"
          timers:
            flush: ""
            timeout: ""
            update: ""
        ripng:
          admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          bfd-liveness: "false"
          check-zero: "false"
          description: ""
          export-limit:
            log-percent: ""
            number: ""
          export-policy: ""
          group:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            bfd-liveness: ""
            check-zero: ""
            description: ""
            export-policy: ""
            group-name: ""
            import-policy: ""
            message-size: ""
            metric-in: ""
            metric-out: ""
            neighbor:
            - admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              bfd-liveness: ""
              check-zero: ""
              description: ""
              export-policy: ""
              import-policy: ""
              interface-name: ""
              message-size: ""
              metric-in: ""
              metric-out: ""
              preference: ""
              receive: ""
              send: ""
              split-horizon: ""
              timers:
                flush: ""
                timeout: ""
                update: ""
              unicast-address:
              - address: ""
            preference: ""
            receive: ""
            send: ""
            split-horizon: ""
            timers:
              flush: ""
              timeout: ""
              update: ""
          import-policy: ""
          message-size: "25"
          metric-in: "1"
          metric-out: "1"
          preference: "100"
          receive: ripng
          send: ripng
          split-horizon: "true"
          timers:
            flush: ""
            timeout: ""
            update: ""
        route-distinguisher: ""
        router-id: ""
        selective-fib: "true"
        service-id: ""
        service-name: ""
        sfm-overload:
          holdoff-time: ""
        sgt-qos:
          dot1p:
            application:
            - apply-groups: ""
              apply-groups-exclude: ""
              dot1p: ""
              dot1p-app-name: ""
          dscp:
            application:
            - apply-groups: ""
              apply-groups-exclude: ""
              dscp: ""
              dscp-app-name: ""
            dscp-map:
            - apply-groups: ""
              apply-groups-exclude: ""
              dscp-name: ""
              fc: ""
        snmp:
          access: "false"
          community:
          - access-permissions: ""
            apply-groups: ""
            apply-groups-exclude: ""
            community-string: ""
            source-access-list: ""
            version: both
        source-address:
          ipv4:
          - address: ""
            application: ""
            apply-groups: ""
            apply-groups-exclude: ""
            interface-name: ""
          ipv6:
          - address: ""
            application: ""
            apply-groups: ""
            apply-groups-exclude: ""
        spoke-sdp:
        - apply-groups: ""
          apply-groups-exclude: ""
          description: ""
          sdp-bind-id: ""
        static-routes:
          apply-groups: ""
          apply-groups-exclude: ""
          hold-down:
            initial: ""
            max-value: ""
            multiplier: ""
          route:
          - apply-groups: ""
            apply-groups-exclude: ""
            backup-tag: ""
            blackhole:
              admin-state: ""
              apply-groups: ""
              apply-groups-exclude: ""
              community: ""
              description: ""
              generate-icmp: "false"
              metric: "1"
              preference: "5"
              prefix-list:
                flag: any
                name: ""
              tag: ""
            community: ""
            grt:
              admin-state: ""
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              metric: "1"
              preference: "5"
            indirect:
            - admin-state: ""
              apply-groups: ""
              apply-groups-exclude: ""
              community: ""
              cpe-check:
              - address: ""
                apply-groups: ""
                apply-groups-exclude: ""
                drop-count: "3"
                interval: "1"
                log: "false"
                padding-size: "56"
              description: ""
              destination-class: ""
              ip-address: ""
              metric: "1"
              preference: "5"
              prefix-list:
                flag: any
                name: ""
              qos:
                forwarding-class: ""
                priority: ""
              source-class: ""
              tag: ""
            interface:
            - admin-state: ""
              apply-groups: ""
              apply-groups-exclude: ""
              community: ""
              cpe-check:
              - address: ""
                apply-groups: ""
                apply-groups-exclude: ""
                drop-count: "3"
                interval: "1"
                log: "false"
                padding-size: "56"
              description: ""
              destination-class: ""
              interface-name: ""
              load-balancing-weight: ""
              metric: "1"
              preference: "5"
              prefix-list:
                flag: any
                name: ""
              qos:
                forwarding-class: ""
                priority: ""
              source-class: ""
              tag: ""
            ip-prefix: ""
            ipsec-tunnel:
            - admin-state: ""
              apply-groups: ""
              apply-groups-exclude: ""
              community: ""
              description: ""
              destination-class: ""
              ipsec-tunnel-name: ""
              metric: "1"
              preference: "5"
              qos:
                forwarding-class: ""
                priority: ""
              source-class: ""
              tag: ""
            next-hop:
            - admin-state: ""
              apply-groups: ""
              apply-groups-exclude: ""
              backup-next-hop:
                address: ""
              bfd-liveness: "false"
              community: ""
              cpe-check:
              - address: ""
                apply-groups: ""
                apply-groups-exclude: ""
                drop-count: "3"
                interval: "1"
                log: "false"
                padding-size: "56"
              description: ""
              destination-class: ""
              ip-address: ""
              load-balancing-weight: ""
              metric: "1"
              preference: "5"
              prefix-list:
                flag: any
                name: ""
              qos:
                forwarding-class: ""
                priority: ""
              source-class: ""
              tag: ""
              validate-next-hop: "false"
            route-type: ""
            tag: ""
        subscriber-interface:
        - admin-state: enable
          apply-groups: ""
          apply-groups-exclude: ""
          description: ""
          fwd-service: ""
          fwd-subscriber-interface: ""
          group-interface:
          - admin-state: enable
            apply-groups: ""
            apply-groups-exclude: ""
            bonding-parameters:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              connection:
              - apply-groups: ""
                apply-groups-exclude: ""
                connection-index: ""
                service: ""
              fpe: ""
              multicast:
                connection: use-incoming
            brg:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              authenticated-brg-only: "false"
              default-brg-profile: ""
            cflowd-parameters:
              sampling:
              - apply-groups: ""
                apply-groups-exclude: ""
                direction: ingress-only
                sample-profile: ""
                sampling-type: ""
                type: ""
            data-trigger:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
            description: ""
            dynamic-routes-track-srrp:
              hold-time: ""
            group-interface-name: ""
            gtp-parameters:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              fpe: ""
            gx-policy: ""
            ingress:
              policy-accounting: ""
            ingress-stats: "false"
            ip-mtu: ""
            ipoe-linking:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              gratuitous-router-advertisement: "false"
              shared-circuit-id: "false"
            ipoe-session:
              admin-state: ""
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              force-auth:
                cid-change: ""
                rid-change: ""
              ipoe-session-policy: ""
              min-auth-interval: infinite
              radius-session-timeout: ""
              sap-session-limit: ""
              session-limit: ""
              stateless-redundancy: "false"
              user-db: ""
            ipv4:
              arp-host:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                host-limit: "1"
                min-auth-interval: "15"
                sap-host-limit: "1"
              dhcp:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                client-applications:
                  dhcp: "true"
                  ppp: "false"
                description: ""
                filter: ""
                gi-address: ""
                lease-populate:
                  l2-header:
                    mac: ""
                  max-leases: "1"
                match-circuit-id: "false"
                offer-selection:
                  client-mac:
                    discover-delay: ""
                    mac-address: ""
                  discover-delay: ""
                  server:
                  - apply-groups: ""
                    apply-groups-exclude: ""
                    discover-delay: ""
                    ipv4-address: ""
                option-82:
                  action: keep
                  circuit-id:
                    ascii-tuple: ""
                    ifindex: ""
                    none: ""
                    sap-id: ""
                    vlan-ascii-tuple: ""
                  remote-id:
                    ascii-string: ""
                    mac: ""
                    none: ""
                  vendor-specific-option:
                    client-mac-address: "false"
                    pool-name: "false"
                    sap-id: "false"
                    service-id: "false"
                    string: ""
                    system-id: "false"
                proxy-server:
                  admin-state: disable
                  emulated-server: ""
                  lease-time:
                    radius-override: "false"
                    value: ""
                python-policy: ""
                relay-proxy:
                  release-update-src-ip: "false"
                  siaddr-override: ""
                server: ""
                src-ip-addr: auto
                trusted: "false"
                user-db: ""
              icmp:
                mask-reply: "true"
                param-problem:
                  admin-state: enable
                  number: "100"
                  seconds: "10"
                redirects:
                  admin-state: enable
                  number: "100"
                  seconds: "10"
                ttl-expired:
                  admin-state: enable
                  number: "100"
                  seconds: "10"
                unreachables:
                  admin-state: enable
                  number: "100"
                  seconds: "10"
              ignore-df-bit: "false"
              neighbor-discovery:
                local-proxy-arp: "true"
                populate: ""
                proxy-arp-policy: ""
                remote-proxy-arp: "false"
                timeout: "14400"
              qos-route-lookup: ""
              urpf-check:
                mode: strict
            ipv6:
              allow-multiple-wan-addresses: "false"
              auto-reply:
                neighbor-solicitation: "false"
                router-solicitation: "false"
              dhcp6:
                apply-groups: ""
                apply-groups-exclude: ""
                filter: ""
                option:
                  apply-groups: ""
                  apply-groups-exclude: ""
                  interface-id:
                    ascii-tuple: ""
                    if-index: ""
                    sap-id: ""
                    string: ""
                  remote-id: "false"
                override-slaac: "false"
                pd-managed-route:
                  next-hop: ipv6
                proxy-server:
                  admin-state: disable
                  client-applications:
                    dhcp: "true"
                    ppp: "false"
                  preferred-lifetime: "3600"
                  rebind-timer: ""
                  renew-timer: ""
                  server-id:
                    apply-groups: ""
                    apply-groups-exclude: ""
                    duid-en-ascii: ""
                    duid-en-hex: ""
                    duid-ll: ""
                  valid-lifetime: "86400"
                python-policy: ""
                relay:
                  admin-state: disable
                  advertise-selection:
                    client-mac:
                      mac-address: ""
                      preference-option:
                        value: ""
                      solicit-delay: ""
                    preference-option:
                      value: ""
                    server:
                    - apply-groups: ""
                      apply-groups-exclude: ""
                      ipv6-address: ""
                      preference-option:
                        value: ""
                      solicit-delay: ""
                    solicit-delay: ""
                  client-applications:
                    dhcp: "true"
                    ppp: "false"
                  description: ""
                  lease-split:
                    admin-state: disable
                    valid-lifetime: "3600"
                  link-address: ""
                  server: ""
                  source-address: ""
                snooping:
                  admin-state: disable
                user-db: ""
                user-ident: mac
              ipoe-bridged-mode: "false"
              neighbor-discovery:
                apply-groups: ""
                apply-groups-exclude: ""
                dad-snooping: "false"
                neighbor-limit: "1"
              qos-route-lookup: ""
              router-advertisements:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                force-mcast: ""
                max-advertisement-interval: "1800"
                min-advertisement-interval: "900"
                options:
                  current-hop-limit: "64"
                  dns:
                    include-rdnss: "false"
                    rdnss-lifetime: "3600"
                  managed-configuration: "false"
                  mtu: not-included
                  other-stateful-configuration: "false"
                  reachable-time: "0"
                  retransmit-timer: "0"
                  router-lifetime: "4500"
                prefix-options:
                  autonomous: "false"
                  on-link: "true"
                  preferred-lifetime: "3600"
                  valid-lifetime: "86400"
              router-solicit:
                admin-state: disable
                apply-groups: ""
                apply-groups-exclude: ""
                inactivity-timer: "300"
                min-auth-interval: "300"
                user-db: ""
              urpf-check:
                mode: strict
            local-address-assignment:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              ipv4:
                client-applications:
                  ipoe: "false"
                  ppp: "false"
                default-pool: ""
                server: ""
              ipv6:
                client-applications:
                  ipoe-slaac: "false"
                  ipoe-wan: "false"
                  ppp-slaac: "false"
                server: ""
            mac: ""
            nasreq-auth-policy: ""
            oper-up-while-empty: "false"
            pppoe:
              admin-state: disable
              anti-spoof: mac-sid
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              dhcp-client:
                client-id: ""
              policy: ""
              python-policy: ""
              sap-session-limit: "1"
              session-limit: "1"
              user-db: ""
            radius-auth-policy: ""
            redundant-interface: ""
            sap:
            - accounting-policy: ""
              admin-state: enable
              anti-spoof: source-ip-and-mac-addr
              apply-groups: ""
              apply-groups-exclude: ""
              calling-station-id: ""
              collect-stats: "false"
              cpu-protection:
                eth-cfm-monitoring:
                  aggregate: ""
                  car: ""
                ip-src-monitoring: ""
                mac-monitoring: ""
                policy-id: ""
              default-host:
                ipv4:
                - address: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  next-hop: ""
                  prefix-length: ""
                ipv6:
                - address: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  next-hop: ""
                  prefix-length: ""
              description: ""
              dist-cpu-protection: ""
              egress:
                agg-rate:
                  limit-unused-bandwidth: "false"
                  queue-frame-based-accounting: "false"
                  rate: ""
                filter:
                  ip: ""
                  ipv6: ""
                qos:
                  policer-control-policy:
                    policy-name: ""
                  qinq-mark-top-only: "false"
                  sap-egress:
                    policy-name: ""
                  scheduler-policy:
                    policy-name: ""
              eth-cfm:
                apply-groups: ""
                apply-groups-exclude: ""
                collect-lmm-fc-stats:
                  fc: ""
                  fc-in-profile: ""
                collect-lmm-stats: "false"
                mep:
                - admin-state: disable
                  ais: "false"
                  alarm-notification:
                    fng-alarm-time: ""
                    fng-reset-time: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  ccm: "false"
                  ccm-ltm-priority: "7"
                  ccm-padding-size: ""
                  csf:
                    multiplier: "3.5"
                  description: ""
                  eth-test:
                    bit-error-threshold: "1"
                    test-pattern:
                      crc-tlv: "false"
                      pattern: all-zeros
                  fault-propagation: ""
                  grace:
                    eth-ed:
                      max-rx-defect-window: ""
                      priority: ""
                      rx-eth-ed: "true"
                      tx-eth-ed: "false"
                    eth-vsm-grace:
                      rx-eth-vsm-grace: "true"
                      tx-eth-vsm-grace: "true"
                  low-priority-defect: mac-rem-err-xcon
                  ma-admin-name: ""
                  md-admin-name: ""
                  mep-id: ""
                  one-way-delay-threshold: "3"
                squelch-ingress-levels: ""
              fwd-wholesale:
                pppoe-service: ""
              host-admin-state: enable
              host-lockout-policy: ""
              igmp-host-tracking:
                apply-groups: ""
                apply-groups-exclude: ""
                expiry-time: ""
                import-policy: ""
                maximum-number-group-sources: ""
                maximum-number-groups: ""
                maximum-number-sources: ""
                router-alert-check: "true"
              ingress:
                filter:
                  ip: ""
                  ipv6: ""
                qos:
                  match-qinq-dot1p: ""
                  policer-control-policy:
                    policy-name: ""
                  sap-ingress:
                    policy-name: ""
                    queuing-type: ""
                  scheduler-policy:
                    policy-name: ""
              lag:
                link-map-profile: ""
                per-link-hash:
                  class: "1"
                  weight: "1"
              monitor-oper-group: ""
              multi-service-site: ""
              oper-group: ""
              sap-id: ""
              static-host:
                ipv4:
                - admin-state: disable
                  ancp-string: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  int-dest-id: ""
                  ip: ""
                  mac: ""
                  managed-route:
                  - prefix: ""
                  rip-policy: ""
                  shcv: {}
                  sla-profile: ""
                  sub-profile: ""
                  subscriber-id:
                    string: ""
                    use-sap-id: ""
                ipv6:
                - admin-state: disable
                  ancp-string: ""
                  apply-groups: ""
                  apply-groups-exclude: ""
                  int-dest-id: ""
                  mac: ""
                  mac-linking: ""
                  managed-route:
                  - apply-groups: ""
                    apply-groups-exclude: ""
                    ipv6-prefix: ""
                    metric: "0"
                  prefix: ""
                  retail-svc-id: ""
                  shcv: {}
                  sla-profile: ""
                  sub-profile: ""
                  subscriber-id:
                    string: ""
                    use-sap-id: ""
                mac-learning:
                  data-triggered: "false"
                  single-mac: "false"
              sub-sla-mgmt:
                admin-state: disable
                defaults:
                  int-dest-id:
                    string: ""
                    top-q-tag: ""
                  sla-profile: ""
                  sub-profile: ""
                  subscriber-id:
                    auto-id: ""
                    sap-id: ""
                    string: ""
                single-sub-parameters:
                  non-sub-traffic:
                    sla-profile: ""
                    sub-profile: ""
                    subscriber-id: ""
                  profiled-traffic-only: "false"
                sub-ident-policy: ""
                subscriber-limit: "1"
            sap-parameters:
              anti-spoof: ip-mac
              apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              sub-sla-mgmt:
                defaults:
                  sla-profile: ""
                  sub-profile: ""
                  subscriber-id:
                    auto-id: ""
                    string: ""
                sub-ident-policy: ""
            shcv-policy: ""
            shcv-policy-ipv4: ""
            shcv-policy-ipv6: ""
            srrp:
            - admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              bfd-liveness:
                apply-groups: ""
                apply-groups-exclude: ""
                dest-ip: ""
                interface-name: ""
                service-name: ""
              description: ""
              gw-mac: ""
              keep-alive-interval: "10"
              message-path: ""
              monitor-oper-group:
                group-name: ""
                priority-step: ""
              one-garp-per-sap: "false"
              policy: ""
              preempt: "true"
              priority: "100"
              send-fib-population-packets: all
              srrp-id: ""
            suppress-aa-sub: "false"
            tos-marking-state: trusted
            type: plain
            wlan-gw:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              gateway-address:
              - address: ""
                apply-groups: ""
                apply-groups-exclude: ""
                purpose:
                  xconnect: "false"
              gateway-router: ""
              l2-ap:
                access-point:
                - admin-state: disable
                  apply-groups: ""
                  apply-groups-exclude: ""
                  encap-type: ""
                  epipe-sap-template: ""
                  sap-id: ""
                auto-sub-id-fmt: include-ap-tags
                default-encap-type: "null"
              lanext:
                max-bd: "131071"
              learn-ap-mac:
                delay-auth: "false"
              mobility:
                hold-time: "5"
                inter-tunnel-type: "false"
                inter-vlan: "false"
                trigger:
                  control: "false"
                  data: "false"
                  iapp: "false"
              oper-down-on-group-degrade: "false"
              tcp-mss-adjust: ""
              tunnel-egress-qos:
                admin-state: disable
                agg-rate-limit: max
                granularity: per-tunnel
                hold-time: ""
                multi-client-only: "false"
                qos: ""
                scheduler-policy: ""
              tunnel-encaps:
                learn-l2tp-cookie: never
              vlan-range:
              - apply-groups: ""
                apply-groups-exclude: ""
                authentication:
                  hold-time: "5"
                  on-control-plane: "false"
                  policy: ""
                  vlan-mismatch-timeout: ""
                data-triggered-ue-creation:
                  admin-state: disable
                  create-proxy-cache-entry:
                    mac-format: 'aa:'
                    proxy-server:
                      name: ""
                      router-instance: ""
                dhcp4:
                  admin-state: disable
                  dns: ""
                  l2-aware-ip-address: ""
                  lease-time:
                    active: "600"
                    initial: "600"
                  nbns: ""
                dhcp6:
                  admin-state: disable
                  apply-groups: ""
                  apply-groups-exclude: ""
                  preferred-lifetime:
                    active: "600"
                    initial: "300"
                  valid-lifetime:
                    active: "600"
                    initial: "300"
                dsm:
                  accounting-policy: ""
                  accounting-update:
                    interval: ""
                  admin-state: disable
                  apply-groups: ""
                  apply-groups-exclude: ""
                  egress:
                    policer: ""
                  ingress:
                    ip-filter: ""
                    policer: ""
                  one-time-redirect:
                    port: "80"
                    url: ""
                http-redirect-policy: ""
                idle-timeout-action: remove
                l2-service:
                  admin-state: disable
                  description: ""
                  service: ""
                nat-policy: ""
                range: ""
                retail-service: ""
                slaac:
                  admin-state: disable
                  apply-groups: ""
                  apply-groups-exclude: ""
                  preferred-lifetime:
                    active: "600"
                    initial: "300"
                  valid-lifetime:
                    active: "600"
                    initial: "300"
                vrgw:
                  admin-state: disable
                  apply-groups: ""
                  apply-groups-exclude: ""
                  brg:
                    authenticated-brg-only: "false"
                    default-brg-profile: ""
                  lanext:
                    access:
                      max-mac: "20"
                      multi-access: "false"
                      policer: ""
                    admin-state: disable
                    apply-groups: ""
                    apply-groups-exclude: ""
                    assistive-address-resolution: "false"
                    bd-mac-prefix: ""
                    mac-translation: "false"
                    network:
                      admin-state: enable
                      max-mac: "20"
                      policer: ""
                xconnect:
                  accounting:
                    mobility-updates: "false"
                    policy: ""
                    update-interval: ""
                  admin-state: disable
                  apply-groups: ""
                  apply-groups-exclude: ""
              wlan-gw-group: ""
            wpp:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              initial:
                sla-profile: ""
                sub-profile: ""
              lease-time: "600"
              portal:
                name: ""
                portal-group: ""
                router-instance: ""
              restore-to-initial-on-disconnect: "true"
              triggered-hosts: "false"
              user-db: ""
          hold-time:
            ipv4:
              down:
                init-only: "false"
                seconds: ""
              up:
                seconds: ""
            ipv6:
              down:
                init-only: "false"
                seconds: ""
              up:
                seconds: ""
          interface-name: ""
          ipoe-linking:
            apply-groups: ""
            apply-groups-exclude: ""
            gratuitous-router-advertisement: "false"
          ipoe-session:
            apply-groups: ""
            apply-groups-exclude: ""
            session-limit: ""
          ipv4:
            address:
            - apply-groups: ""
              apply-groups-exclude: ""
              gateway: ""
              holdup-time: ""
              ipv4-address: ""
              populate-host-routes: "false"
              prefix-length: ""
              track-srrp: ""
            allow-unmatching-subnets: "false"
            arp-host:
              admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              host-limit: ""
            default-dns: ""
            dhcp:
              admin-state: enable
              apply-groups: ""
              apply-groups-exclude: ""
              client-applications:
                dhcp: "true"
                ppp: "false"
              description: ""
              gi-address: ""
              lease-populate:
                max-leases: ""
              offer-selection:
                client-mac:
                  discover-delay: ""
                  mac-address: ""
                discover-delay: ""
                server:
                - apply-groups: ""
                  apply-groups-exclude: ""
                  discover-delay: ""
                  ipv4-address: ""
              option-82:
                vendor-specific-option:
                  client-mac-address: "false"
                  sap-id: "false"
                  service-id: "false"
                  string: ""
                  system-id: "false"
              proxy-server:
                admin-state: disable
                emulated-server: ""
                lease-time:
                  radius-override: "false"
                  value: ""
              python-policy: ""
              relay-proxy:
                release-update-src-ip: "false"
                siaddr-override: ""
              server: ""
              src-ip-addr: auto
              virtual-subnet: "false"
            export-host-routes: "false"
            unnumbered:
              ip-address: ""
              ip-int-name: ""
          ipv6:
            address:
            - apply-groups: ""
              apply-groups-exclude: ""
              host-type: pd
              ipv6-address: ""
              prefix-length: ""
            allow-multiple-wan-addresses: "false"
            allow-unmatching-prefixes: "false"
            default-dns: ""
            delegated-prefix-length: "64"
            dhcp6:
              apply-groups: ""
              apply-groups-exclude: ""
              override-slaac: "false"
              pd-managed-route:
                next-hop: ipv6
              proxy-server:
                admin-state: disable
                client-applications:
                  dhcp: "true"
                  ppp: "false"
                preferred-lifetime: "3600"
                rebind-timer: ""
                renew-timer: ""
                server-id:
                  apply-groups: ""
                  apply-groups-exclude: ""
                  duid-en-ascii: ""
                  duid-en-hex: ""
                  duid-ll: ""
                valid-lifetime: "86400"
              python-policy: ""
              relay:
                admin-state: disable
                advertise-selection:
                  client-mac:
                    mac-address: ""
                    preference-option:
                      value: ""
                    solicit-delay: ""
                  preference-option:
                    value: ""
                  server:
                  - apply-groups: ""
                    apply-groups-exclude: ""
                    ipv6-address: ""
                    preference-option:
                      value: ""
                    solicit-delay: ""
                  solicit-delay: ""
                client-applications:
                  dhcp: "true"
                  ppp: "false"
                description: ""
                lease-split:
                  admin-state: disable
                  valid-lifetime: "3600"
                link-address: ""
                server: ""
                source-address: ""
            ipoe-bridged-mode: "false"
            link-local-address:
              address: ""
            prefix:
            - apply-groups: ""
              apply-groups-exclude: ""
              holdup-time: ""
              host-type: pd
              ipv6-prefix: ""
              track-srrp: ""
            router-advertisements:
              admin-state: disable
              apply-groups: ""
              apply-groups-exclude: ""
              force-mcast: ""
              max-advertisement-interval: "1800"
              min-advertisement-interval: "900"
              options:
                current-hop-limit: "64"
                dns:
                  include-rdnss: "false"
                  rdnss-lifetime: "3600"
                managed-configuration: "false"
                mtu: not-included
                other-stateful-configuration: "false"
                reachable-time: "0"
                retransmit-timer: "0"
                router-lifetime: "4500"
              prefix-options:
                autonomous: "false"
                on-link: "true"
                preferred-lifetime: "3600"
                valid-lifetime: "86400"
            router-solicit:
              apply-groups: ""
              apply-groups-exclude: ""
              inactivity-timer: "300"
          local-address-assignment:
            admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            ipv4:
              client-applications:
                ppp: "false"
              default-pool: ""
              server: ""
            ipv6:
              client-applications:
                ipoe-slaac: "false"
                ipoe-wan: "false"
                ppp-slaac: "false"
              server: ""
          pppoe:
            apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            session-limit: "1"
          private-retail-subnets: "false"
          wan-mode: mode64
          wlan-gw:
            apply-groups: ""
            apply-groups-exclude: ""
            pool-manager:
              apply-groups: ""
              apply-groups-exclude: ""
              dhcp6-client:
                dhcpv4-nat:
                  admin-state: disable
                  link-address: '::'
                  pool-name: ""
                ia-na:
                  admin-state: disable
                  link-address: '::'
                  pool-name: ""
                lease-query:
                  max-retries: "2"
                servers: ""
                slaac:
                  admin-state: disable
                  link-address: '::'
                  pool-name: ""
                source-ip: use-interface-ip
              watermarks:
                high: "95"
                low: "90"
              wlan-gw-group: ""
            redundancy:
              admin-state: disable
              export: ""
              monitor: ""
        ttl-propagate:
          local: use-base
          transit: use-base
        twamp-light:
          apply-groups: ""
          apply-groups-exclude: ""
          reflector:
            admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            description: ""
            prefix:
            - apply-groups: ""
              apply-groups-exclude: ""
              description: ""
              ip-prefix: ""
            udp-port: ""
        video-interface:
        - accounting-policy: ""
          address:
          - ip-address: ""
          adi:
            scte30:
              ad-server:
              - address: ""
              local-address:
                apply-groups: ""
                apply-groups-exclude: ""
                control: ""
                data: ""
          admin-state: disable
          apply-groups: ""
          apply-groups-exclude: ""
          channel:
          - apply-groups: ""
            apply-groups-exclude: ""
            channel-name: ""
            description: ""
            mcast-address: ""
            scte35-action: forward
            source: ""
            zone-channel:
            - adi-channel-name: ""
              apply-groups: ""
              apply-groups-exclude: ""
              zone-mcast-address: ""
              zone-source: ""
          cpu-protection: ""
          description: ""
          interface-name: ""
          multicast-service: ""
          output-format: rtp-udp
          rt-client:
            apply-groups: ""
            apply-groups-exclude: ""
            src-address: ""
          video-sap:
            apply-groups: ""
            apply-groups-exclude: ""
            egress:
              apply-groups: ""
              apply-groups-exclude: ""
              filter:
                ip: ""
              qos:
                policy-name: ""
            ingress:
              apply-groups: ""
              apply-groups-exclude: ""
              filter:
                ip: ""
              qos:
                policy-name: ""
            video-group-id: ""
        vprn-type: regular
        vrf-export:
          apply-groups: ""
          apply-groups-exclude: ""
          policy: ""
        vrf-import:
          apply-groups: ""
          apply-groups-exclude: ""
          policy: ""
        vrf-target:
          community: ""
          export-community: ""
          import-community: ""
        vxlan:
          tunnel-termination:
          - apply-groups: ""
            apply-groups-exclude: ""
            fpe-id: ""
            ip-address: ""
        weighted-ecmp: "false"
        wlan-gw:
          apply-groups: ""
          apply-groups-exclude: ""
          distributed-subscriber-mgmt:
            apply-groups: ""
            apply-groups-exclude: ""
            ipv6-tcp-mss-adjust: ""
          mobility-triggered-accounting:
            admin-state: disable
            hold-down: ""
            include-counters: "false"
          xconnect:
            admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            tunnel-source-ip: ""
            wlan-gw-group: ""
        wpp:
          admin-state: disable
          apply-groups: ""
          apply-groups-exclude: ""
          portal:
          - ack-auth-retry-count: "5"
            address: ""
            admin-state: disable
            apply-groups: ""
            apply-groups-exclude: ""
            name: ""
            ntf-logout-retry-count: "5"
            port-format: standard
            retry-interval: "2000"
            secret: ""
            version: "1"
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
          --file cisco/xr/721/Cisco-IOS-XR-um-router-bgp-cfg.yang \
          --file cisco/xr/721/Cisco-IOS-XR-ipv4-bgp-oper.yang \
          --dir standard/ietf \
          set-request \
          --path / # todo
```

#### Juniper
YANG repo: [Juniper/yang](https://github.com/Juniper/yang)

Clone the Juniper YANG repository and change into the release directory:

```bash
git clone https://github.com/Juniper/yang
cd yang/20.3/20.3R1
```

#### Arista
YANG repo: [aristanetworks/yang](https://github.com/aristanetworks/yang)

Arista uses a subset of OpenConfig modules and does not provide IETF modules inside their repo. So make sure you have IETF models available so you can reference it, a `openconfig/public` is a good candidate.

Clone the Arista YANG repo:

```bash
git clone https://github.com/aristanetworks/yang
cd yang
```
