The Consul target loader discovers gNMI targets registered as service instances in a Consul Server.

The loader watches services registered in Consul defined by a service name and optionally a set of tags.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/target_discovery.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Ftarget_discovery.drawio" async></script>

### Services watch

When at least one service name is set, gNMIc consul loader will watch the instances registered under that service name and build a target configuration using the service ID as the target name and the registered address and port as the target address.

The remaining configuration can be set under the service name definition.

```yaml
loader:
  type: consul
  services:
    - name: cluster1-gnmi-server
      config:
        insecure: true
        username: admin
        password: admin
```

### Configuration

```yaml
loader:
  type: consul
  # address of the loader server
  address: localhost:8500
  # Consul Data center, defaults to dc1
  datacenter: dc1
  # Consul username, to be used as part of HTTP basicAuth
  username:
  # Consul password, to be used as part of HTTP basicAuth
  password:
  # Consul Token, is used to provide a per-request ACL token which overrides the agent's default token
  token:
  # the key prefix to watch for targets configuration, defaults to "gnmic/config/targets"
  key-prefix: gnmic/config/targets
  # if true, registers consulLoader prometheus metrics with the provided
  # prometheus registry
  enable-metrics: false
  # list of services to watch and derive target configurations from.
  services:
      # name of the Consul service
    - name:
      # a list of strings to further filter the service instances
      tags: 
      # configuration map to apply to target discovered from this service
      config:
  # list of actions to run on target discovery
  on-add:
  # list of actions to run on target removal
  on-delete:
  # variable dict to pass to actions to be run
  vars:
  # path to variable file, the variables defined will be passed to the actions to be run
  # values in this file will be overwritten by the ones defined in `vars`
  vars-file:
```
