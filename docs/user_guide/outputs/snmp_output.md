`gnmic` supports generating SNMP traps based on received gNMI updates.

This output type is useful when trying to integrate legacy systems that ingest SNMP traps with more modern telemetry/alarms stacks.

Only SNMPv2c is supported.

## Configuration

The SNMP output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  # the output name
  snmp_trap: 
    # the output type
    type: snmp
    # the traps destination address
    address:
    # the trap destination port, defaults to 162
    port: 162
    # the SNMP trap community
    community: public
    # duration, wait time before the first trap evaluation.
    # defaults to 5s and minimum allowed value is 5s.
    start-delay: 5s
    # traps definition
    traps:
        # if true, the SNMP message generated is an inform request, not a trap.
      - inform: false
        # trap trigger definition,
        # the trigger section of the trap defines which received path trigger the trap
        # as well as the variable binding to append to it.
        trigger:
          # xpath, if present in the received event message, the trap is triggered
          path:
          # a jq script that is executed with the trigger event message as input.
          # must return a valid OID.
          oid:
          # a static string, defining the type of the OID value,
          # one of: bool, int, bitString, octetString, null, objectID, objectDescription,
          # ipAddress, counter32, gauge32, timeTicks, opaque, nsapAddress, counter64, 
          # uint32, opaqueFloat, opaqueDouble
          type:
          # a jq script that is executed with the trigger event message as input.
          # must return a value matching the above configured type.
          value:
        # trap variable bindings definition,
        # the bindings section defines the extra variable bindings to append to the trap.
        # multiple bindings can be defined here.
        bindings:
            # A jq script that is executed with the trigger message as input.
            # Must return a valid xpath.
            # The local cache is queried using the resulting xpath, the resulting event message is used 
            # as input to execute the below oid and value jq scripts
          - path:
            # A jq script that is executed with the message obtained from the cache as input.
            # must return a valid OID. 
            oid:
            # a static string, defining the type of the OID value,
            # one of: bool, int, bitString, octetString, null, objectID, objectDescription,
            # ipAddress, counter32, gauge32, timeTicks, opaque, nsapAddress, counter64, 
            # uint32, opaqueFloat, opaqueDouble
            type:
            # A jq script that is executed with the message obtained from the cache as input.
            # must return a value matching the above configured type.
            value:
```

## How does it work?

The SNMP output stores each received update message in a local cache (1.a), then checks if the message should trigger any of the configured traps (1.b).

If the received message triggers a trap (2), an SNMP variable binding is generated from the trap `trigger` configuration section (`OID`, `type` and `value`) based on the triggering event.
The `OID` and `value` can be [jq](https://github.com/itchyny/gojq) scripts.

Then (3) for each configured binding, the configured `path` (`jq` script) is rendered based on the triggering event then used to retrieve an event message from the cache, that message is then used to generate the variable binding (`OID`, `type` and `value`).

Once all bindings are generated, a `sysUpTimeInstance` (OID=`1.3.6.1.2.1.1.3.0`) binding is prepended to the PDU list of the trap, its value is the number of seconds since `gNMIc` SNMP output startup.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/openconfig/gnmic/diagrams/diagrams/snmp_output.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fopenconfig%2Fgnmic%2Fdiagrams%2Fsnmp_output.drawio" async></script>

## Metrics

The SNMP output exposes 4 Prometheus metrics:

- Number of failed trap generation

- Number of SNMP trap sending failures

- SNMP trap generation duration in ns

```text
gnmic_snmp_output_number_of_snmp_trap_failed_generation{name="snmp_trap",reason="",trap_index="0"} 0
gnmic_snmp_output_number_of_snmp_trap_sent_fail_total{name="snmp_trap",reason="",trap_index="0"} 0
gnmic_snmp_output_number_of_snmp_traps_sent_total{name="snmp_trap",trap_index="0"} 114
gnmic_snmp_output_snmp_trap_generation_duration_ns{name="snmp_trap",trap_index="0"} 380215
```

## Examples

### interface operational state trap

The below example generates an SNMPV2 trap whenever the operational state of an interface changes (`ifOperStatus`).

It adds `sysName`, `ifAdminStatus` and `ifIndex` variable bindings to the trap before sending it out.

```yaml
username: admin
password: admin
skip-verify: true

targets:
  clab-snmp-srl1:
  clab-snmp-srl2:

subscriptions:
  sub1:
    paths:
      - /interface/admin-state
      - /interface/oper-state
      - /interface/ifindex
      - /system/name/host-name
    stream-mode: on-change
    encoding: ascii

outputs:
  snmp_trap:
    type: snmp
    address: snmptrap.server
    # port: 162
    # community: public
    traps:
      - trigger:
          path: /interface/oper-state # static path
          oid: '".1.3.6.1.2.1.2.2.1.8"' # ifOperStatus
          type: int
          value: if (.values."/interface/oper-state" == "up") 
                  then 1 
                  else 2 
                  end
        bindings:         
          - path: '"/system/name/host-name"' # jq script
            oid: '".1.3.6.1.2.1.1.5"' # sysName
            type: octetString
            value: '.values."/system/name/host-name"'

          - path: '"/interface[name="+.tags.interface_name+"]/admin-state"' # jq script
            oid: '".1.3.6.1.2.1.2.2.1.7"' # ifAdminStatus
            type: int
            value: if (.values."/interface/admin-state" == "enable") 
                    then 1 
                    else 2 
                    end

          - path: '"/interface[name="+.tags.interface_name+"]/ifindex"' # jq script
            oid: '".1.3.6.1.2.1.2.2.1.1"' # ifIndex
            type: int
            value: '.values."/interface/ifindex" | tonumber' # jq script
```
