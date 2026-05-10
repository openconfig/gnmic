The Consul target loader discovers gNMI targets registered as service instances in a Consul Server.

The loader watches services registered in Consul defined by a service name and optionally a set of tags.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/openconfig/gnmic/diagrams/diagrams/target_discovery.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Ftarget_discovery.drawio" async></script>

### Services watch

When at least one service name is set, gNMIc consul loader will watch the instances registered under that service name and build a target configuration using the service ID as the target name and the registered address and port as the target address.

The remaining configuration can be set under the service name definition.

```yaml
loader:
  type: consul
  services:
    - name: cluster1-gnmi-server
      filter: Service.Meta.environment == "production"
      config:
        insecure: true
        username: admin
        password: admin
```

### Templating with Consul

It is possible to set the target name to something other than the Consul Service ID using the `name` field under the config. The target name can be customized using [Go Templates](https://golang.org/pkg/text/template/).

In addition to setting the target name, it is also possible to use Go Templates on `event-tags` as well.

The templates use the Service under Consul, so access to things like `ID`, `Tags`, `Meta`, etc. are all available.

```yaml
loader:
  type: consul
  services:
    - name: cluster1-gnmi-server
      config:
        name: "{{.Meta.device}}"
        event-tags:
            location: "{{.Meta.site_name}}"
            model: "{{.Meta.device_type}}"
            tag-1: "{{.Meta.tag_1}}"
            boring-static-tag: "hello"
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
      # optional go-bexpr filter evaluated by Consul in addition to tags
      filter:
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

### Filtering services

Each service entry can also define a [`filter`](https://developer.hashicorp.com/consul/api-docs/features/filtering) that Consul evaluates before returning results. Filters use [go-bexpr syntax](https://github.com/HashiCorp/go-bexpr) and are applied in addition to the `tags` list (both conditions must match).

```yaml
loader:
  type: consul
  services:
    - name: cluster1-gnmi-server
      tags: ["gnmic", "network-device"]
      filter: Service.Meta.environment == "production" && Node.Datacenter == "dc1"
```

Use filters to keep existing tag-based configs working while narrowing the results with metadata such as health status, service meta fields, or node attributes.
```
