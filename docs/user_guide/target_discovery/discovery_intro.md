## Introduction

`gnmic` supports dynamic loading of gNMI targets from external systems.
This feature allows adding and deleting gNMI targets without the need to restart `gnmic`.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/target_discovery.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Ftarget_discovery.drawio" async></script>

Depending on the discovery method, `gnmic` will either:

- Subscribe to changes on the remote system,
- Or poll the defined targets from the remote systems.
  
When a change is detected, the new targets are added and the corresponding subscriptions are immediately established.
The removed targets are deleted together with their subscriptions.

Actions can be run on target discovery (on-add or on-delete), this can be useful to add initial configurations to target ahead of gNMI subscriptions or run checks before subscribing.
In the case of on-add actions,

!!! notes
    1. Only one discovery type is supported at a time.

    2. Target updates are not supported, delete and re-add is the way to update a target configuration.

## Discovery types

Four types of target discovery methods are supported:

### [File Loader](./file_discovery.md)

Watches changes to a local file containing gNMI targets definitions.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/target_discovery.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Ftarget_discovery.drawio" async></script>

### [Consul Server Loader](./consul_discovery.md)

Subscribes to Consul KV key prefix changes, the keys and their value represent a target configuration fields.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/target_discovery.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Ftarget_discovery.drawio" async></script>

### [Docker Engine Loader](./docker_discovery.md)

Polls containers from a Docker Engine host matching some predefined criteria (docker filters).

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:3,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/target_discovery.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Ftarget_discovery.drawio" async></script>

### [HTTP Loader](./http_discovery.md)

Queries an HTTP endpoint periodically, expected a well formatted JSON dict of targets configurations.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:4,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/target_discovery.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Ftarget_discovery.drawio" async></script>

## Running actions on discovery

All actions support fields `on-add` and `on-delete` which take a list of predefined action names that will be run sequentially on target discovery or deletion.

The below configuration example defines 3 actions `configure_interfaces`, `configure_subinterfaces` and `configure_network_instance` which will run when the `docker` loader discovers a target with label `clab-node-kind=srl`

``` yaml
loader:
  type: docker
  filters:
    - containers:
      - label: clab-node-kind=srl
      config:
        skip-verify: true
        username: admin
        password: admin
  on-add:
    - configure_interfaces
    - configure_subinterfaces
    - configure_network_instances
 
actions:
  configure_interfaces:
    name: configure_interfaces
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
  configure_subinterfaces:
    name: configure_subinterfaces
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
  configure_network_instances:
    name: configure_network_instances
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
