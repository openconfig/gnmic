# Actions

`gNMIc` supports running actions as result of an event, possible triggering events are:

- A gNMI SubscribeResponse or GetReponse message is received and matches certain criteria.
- A target is discovered or deleted by a target loader.

There are 4 types of actions:

- [http](#http-action): build and send an HTTP request
- [gNMI](#gnmi-action): run a Get, Set or Subscribe ONCE gNMI RPC as a gNMI client
- [template](#template-action): execute a Go template against the received input
- [script](#script-action): run arbitrary shell scripts/commands.

The actions are executed in sequence.

An action can use the result of any previous action as one of it inputs using the [Go Template](https://golang.org/pkg/text/template/) syntax `{{ .Env.$action_name }}` or `{{ index .Env "$action_name"}}`

### HTTP Action

Using the `HTTP action` you can send an HTTP request to a server.

The request body can be customized using [Go Templates](https://golang.org/pkg/text/template/) that take the event message or the discovered target as input.

```yaml
actions:
  counter1_alert:
    # action type
    type: http
    # HTTP method
    method: POST
    # target url, can be a go template
    url: http://remote-server:8080/
    # http headers to add to the request
    headers: 
      content-type: application/text
    # http request timeout
    timeout: 5s
    # go template used to build the request body.
    # if left empty the whole event message is added as a json object to the request's body
    body: '"counter1" crossed threshold, value={{ index .Values "counter1" }}'
    # enable extra logging
    debug: false
```

### gNMI Action

Using the `gNMI action` you can trigger a gNMI Get, Set or Subscribe ONCE RPC.

Just like the `HTTP action` the RPC fields can be customized using [Go Templates](https://golang.org/pkg/text/template/)

```yaml
actions:
  my_gnmi_action:
    # action type
    type: gnmi
    # gNMI rpc, defaults to `get`, 
    # if `set` is used it will default to a set update.
    # to trigger a set replace, use `set-replace`.
    # `subscribe` is always a subscribe with mode=ONCE
    # possible values: `get`, `set`, `set-update`, `set-replace`, `set-delete`, `sub`, `subscribe`
    rpc: set
    # the target router, it defaults to the value in tag "source"
    # the value `all` means all known targets
    target: '{{ index .Event.Tags "source" }}'
    # paths templates to build xpaths
    paths:
      - | 
        {{ if eq ( index .Event.Tags "interface_name" ) "ethernet-1/1"}}
          {{$interfaceName := "ethernet-1/2"}}
        {{else}}
          {{$interfaceName := "ethernet-1/1"}}
        {{end}}
        /interfaces/interface[name={{$interfaceName}}]/admin-state
    # values templates to build the values in case of set-update or set-replace
    values:
      - "enable"
    # data-type in case of get RPC, one of: ALL, CONFIG, STATE, OPERATIONAL
    data-type: ALL
    # gNMI encoding, defaults to json
    encoding: json
    # debug, enable extra logging
    debug: false
```

### Template Action

The `Template action` allows to combine different data sources and produce custom payloads to be writen to a remote server or simply to a file.

The template is a Go Template that is executed against the `Input` message that triggered the action,
any variable defined by the trigger processor
as well as the results of any previous action.

**Data**                      | **Template syntax**                                           |
----------------------------- | --------------------------------------------------------------|
**Input Messge**              | `{{ .Input }}`                                                |
**Trigger Variables**         | `{{ .Vars }}`                                                 |
**Previous actions results**  | `{{ .Env.$action_name }}` or `{{ index .Env "$action_name"}}` |

```yaml
actions:
  awesome_template:
    # action type
    type: template
    # template string, if not present template-file applies.
    template: '{{ . }}'
    # path to a file, or a glob.
    # applies only if `.template `is not set.
    # if not template and template-file are not set, 
    # the default template `{{ . }}` is used.
    template-file:
    # string, either `stdout` or a path to a file
    # the result of executing to template will be written to the file
    # specified by .output
    output:
    # debug, enable extra logging
    debug: false
```

### Script Action

The `Script action` allows to run arbitrary scripts as a result of an event trigger.

The commands to be executed can be specified using the field `command`, e.g:

```yaml
actions:
  weather:
    type: script
    shell: /bin/bash
    command: | 
      curl wttr.in
      curl cheat.sh
```

Or using the field `file`, e.g:

```yaml
actions:
  exec:
    type: script
    file: ./my_executable_script.sh
```

When using `command`, the shell interpreter can be set using `shell` field. Otherwise it defaults to `/bin/bash`.

### Examples

#### Add basic configuration to targets upon discovery

Referencing Actions under a target loader allows to run then in sequence when a target is discovered.

This allows to add some basic configuration to a target upon discovery before starting the gNMI subscriptions

In the below example, a `docker` loader is defined. It discovers Docker containers with label `clab-node-kind=srl` and adds them as gNMI targets.
Before the targets are added to the target's list for subscriptions, a list of actions are executed: `config_interfaces`, `config_subinterfaces` and `config_network_instances`

```yaml
username: admin
password: admin
skip-verify: true
encoding: ascii
log: true

subscriptions:
  sub1:
    paths:
      - /interface/statistics
      - /network-instance/statistics

loader:
  type: docker
  filters:
    - containers:
      - label: clab-node-kind=srl

  on-add:
    - config_interfaces
    - config_sub_interfaces
    - config_netins

outputs:
  out:
    type: file
    format: event
    filename: /path/to/file

actions:
  config_interfaces:
    name: config_interfaces
    type: gnmi
    target: '{{ .Input }}'
    rpc: set
    encoding: json_ietf
    debug: true
    paths:
      - /interface[name=ethernet-1/1]/admin-state
      - /interface[name=ethernet-1/2]/admin-state 
    values:
      - enable
      - enable
  config_subinterfaces:
    name: config_subinterfaces
    type: gnmi
    target: '{{ .Input }}'
    rpc: set
    encoding: json_ietf
    debug: true
    paths:
      - /interface[name=ethernet-1/1]/subinterface[index=0]/admin-state
      - /interface[name=ethernet-1/2]/subinterface[index=0]/admin-state 
    values:
      - enable
      - enable
  config_network_instances:
    name: config_network_instances
    type: gnmi
    target: '{{ .Input }}'
    rpc: set
    encoding: json_ietf
    debug: true
    paths:
      - /network-instance[name=default]/admin-state
      - /network-instance[name=default]/interface
      - /network-instance[name=default]/interface
    values:
      - enable
      - '{"name": "ethernet-1/1.0"}'
      - '{"name": "ethernet-1/2.0"}'
```

#### Clone a network topology and deploy it using containerlab

Using lldp neighbor information it's possible to build a containerlab topology using `gnmic` actions.

In the below confoguration file, an event processor called `clone-topology` is defined.

When triggered it will run a series of actions to gather information (chassis type, lldp neighbors, configuration,...) from the defined targets.

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
    #debug: true
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
              {{ $n | strings.TrimPrefix "clab-test1-" }}:
                type: {{ $type }}
                startup-config: {{ print $n ".json"}}
          {{- end }}
          
            links:
          {{- range $n, $m := .Env.lldp }}
            {{- range $rsp := $m }}
              {{- range $ev := $rsp }}
                {{- if index $ev.values "/srl_nokia-system:system/srl_nokia-lldp:lldp/interface/neighbor/system-name" }}
                {{- $node1 := $ev.tags.source | strings.TrimPrefix "clab-test1-" }}
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
