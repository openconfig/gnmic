
The `event-trigger` processor takes event messages as input and triggers a list of actions (sequentially) if a configured condition evaluates to `true`.

The condition is evaluated using the the Golang implementation of [jq](https://github.com/itchyny/gojq) with the event message as a `json` input.

`jq` [tutorial](https://stedolan.github.io/jq/tutorial/)

`jq` [manual](https://stedolan.github.io/jq/manual/)

`jq` [playground](https://jqplay.org/)

Examples of conditions:

- The below expression checks if the value named `counter1` has a value higher than 90

```bash
.values["counter1"] > 90
```

- This expression checks if the event name is `sub1`, that the tag `source` is equal to `r1:57400`

```bash
.name == "sub1" and .tags["source"] == "r1:57400" 
```

The trigger can be monitored over a configurable window of time (default 1 minute), during which only a certain number of occurrences (default 1) trigger the actions execution.

The action types availabe can be found [here](../actions/actions.md)

```yaml
processors:
  # processor name
  my_trigger_proc: # 
    # processor type
    event-trigger:
      # trigger condition
      condition: '.values["counter1"] > 90'
      # minimum number of condition occurrences within the configured window 
      # required to trigger the action
      min-occurrences: 1
      # max number of times the action is triggered within the configured window
      max-occurrences: 1
      # window of time during which max-occurrences need to 
      # be reached in order to trigger the action
      window: 60s
      # async, bool. default false.
      # If true the trigger is executed in the background and the triggering
      # message is passed to the next procesor. Otherwise it blocks until the trigger returns
      async: false
      # a dictionary of variables that is passed to the actions
      # and can be accessed in the actions templates using `.Vars`
      vars:
      # path to a file containing variables passed to the actions
      # the variable in the `vars` field override the ones read from the file.
      vars-file: 
      # list of actions to be executed
      actions:
        - counter_alert
```

### Examples

#### Alerting when a threshold is crossed

The below example triggers an HTTP GET to `http://remote-server:p8080/${router_name}` if the value of counter "counter1" crosses 90 twice within 2 minutes.

```yaml
processors:
  my_trigger_proc:
    event-trigger:
      condition: '.values["counter1"] > 90'
      min-occurrences: 1
      max-occurrences: 2
      window: 120s
      async: true
      actions:
        - alert

actions:
  alert:
    name: alert
    type: http
    method: POST
    url: http://remote-server:8080/{{ index .Tags "source" }}
    headers: 
      content-type: application/text
    timeout: 5s
    body: '"counter1" crossed threshold, value={{ index .Values "counter1" }}'
```

#### Enabling backup interface

The below example shows a trigger that enables a router interface if another interface's operational status changes to "down".

```yaml
processors:
  interface_watch: # 
    event-trigger:
      debug: true
      condition: '(.tags.interface_name == "ethernet-1/1" or .tags.interface_name == "ethernet-1/2") and .values["/srl_nokia-interfaces:interface/oper-state"] == "down"'
      actions:
      - enable_interface

actions:
  enable_interface:
    name: my_gnmi_action
    type: gnmi
    rpc: set
    target: '{{ index .Event.Tags "source" }}'
    paths:
      - |
        {{ $interfaceName := "" }}
        {{ if eq ( index .Event.Tags "interface_name" ) "ethernet-1/1"}}
        {{$interfaceName = "ethernet-1/2"}}
        {{ else if eq ( index E.vent.Tags "interface_name" ) "ethernet-1/2"}}
        {{$interfaceName = "ethernet-1/1"}}
        {{end}}
        /interface[name={{$interfaceName}}]/admin-state
    values:
      - "enable"
    encoding: json_ietf
    debug: true
```

#### Clone a network topology and deploy it using containerlab

Using lldp neighbor information it's possible to build a [containerlab](https://containerlab.srlinux.dev) topology using `gnmic` actions.

In the below configuration file, an event processor called `clone-topology` is defined.

When triggered it runs a series of actions to gather information (chassis type, lldp neighbors, configuration,...) from the defined targets.

It then builds a containerlab topology from a defined template and the gathered info, writes it to a file and runs a `clab deploy` command.

```yaml
username: admin
password: admin
skip-verify: true
encoding: json_ietf
# log: true

targets:
  srl1:
  srl2:
  srl3:

processors:
  clone-topology:
    event-trigger:
      # debug: true
      actions:
        - chassis  
        - lldp  
        - read_config  
        - write_config 
        - clab_topo         
        - deploy_topo

actions:
  chassis:
    name: chassis
    type: gnmi
    target: all
    rpc: sub
    encoding: json_ietf
    #debug: true
    format: event
    paths:
      - /platform/chassis/type
  
  lldp:
    name: lldp
    type: gnmi
    target: all
    rpc: sub
    encoding: json_ietf
    #debug: true
    format: event
    paths:
      - /system/lldp/interface[name=ethernet-*]
  
  read_config:
    name: read_config
    type: gnmi
    target: all
    rpc: get
    data-type: config
    encoding: json_ietf
    #debug: true
    paths:
      - /
  
  write_config:
    name: write_config
    type: template
    template: |
      {{- range $n, $m := .Env.read_config }}
      {{- $filename := print $n  ".json"}}
          {{ file.Write $filename (index $m 0 "updates" 0 "values" "" | data.ToJSONPretty "  " ) }}
          {{- end }}
        #debug: true
  
  clab_topo:
    name: clab_topo
    type: template
    # debug: true
    output: gnmic.clab.yaml
    template: |
      name: gNMIc-action-generated
  
      topology:
        defaults:
          kind: srl
        kinds:
          srl:
            image: ghcr.io/nokia/srlinux:latest
  
        nodes:
      {{- range $n, $m := .Env.lldp }}
        {{- $type := index $.Env.chassis $n 0 0 "values" "/srl_nokia-platform:platform/srl_nokia-platform-chassis:chassis/type" }}
        {{- $type = $type | strings.ReplaceAll "7220 IXR-D1" "ixrd1" }}
        {{- $type = $type | strings.ReplaceAll "7220 IXR-D2" "ixrd2" }}
        {{- $type = $type | strings.ReplaceAll "7220 IXR-D3" "ixrd3" }}
        {{- $type = $type | strings.ReplaceAll "7250 IXR-6" "ixr6" }}
        {{- $type = $type | strings.ReplaceAll "7250 IXR-10" "ixr10" }}
        {{- $type = $type | strings.ReplaceAll "7220 IXR-H1" "ixrh1" }}
        {{- $type = $type | strings.ReplaceAll "7220 IXR-H2" "ixrh2" }}
        {{- $type = $type | strings.ReplaceAll "7220 IXR-H3" "ixrh3" }}
          {{ $n | strings.TrimPrefix "clab-" }}:
            type: {{ $type }}
            startup-config: {{ print $n ".json"}}
      {{- end }}
      
        links:
      {{- range $n, $m := .Env.lldp }}
        {{- range $rsp := $m }}
          {{- range $ev := $rsp }}
            {{- if index $ev.values "/srl_nokia-system:system/srl_nokia-lldp:lldp/interface/neighbor/system-name" }}
            {{- $node1 := $ev.tags.source | strings.TrimPrefix "clab-" }}
            {{- $iface1 := $ev.tags.interface_name | strings.ReplaceAll "ethernet-" "e" | strings.ReplaceAll "/" "-" }}
            {{- $node2 := index $ev.values "/srl_nokia-system:system/srl_nokia-lldp:lldp/interface/neighbor/system-name" }}
            {{- $iface2 := index $ev.values "/srl_nokia-system:system/srl_nokia-lldp:lldp/interface/neighbor/port-id" | strings.ReplaceAll "ethernet-" "e" | strings.ReplaceAll "/" "-" }}
              {{- if lt $node1 $node2 }}
          - endpoints: ["{{ $node1 }}:{{ $iface1 }}", "{{ $node2 }}:{{ $iface2 }}"]
              {{- end }}
            {{- end }}
          {{- end }}
        {{- end }}
      {{- end }}
    
  deploy_topo:  
    name: deploy_topo
    type: script
    command: sudo clab dep -t gnmic.clab.yaml --reconfigure
    debug: true
```

The above described processor can be triggered with the below command:

```bash
gnmic --config clone.yaml get --path /system/name --processor clone-topology
```
