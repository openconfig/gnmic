`gnmic` supports exposing gnmi updates on a prometheus server, for a prometheus client to scrape.

A Prometheus output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    type: prometheus # require
    # address to listen on for incoming scrape requests
    listen: :9804 
    # path to query to get the metrics
    path: /metrics 
    # maximum lifetime of metrics in the local cache, #
    # a zero value defaults to 60s, a negative duration (e.g: -1s) disables the expiration
    expiration: 60s 
    # a string to be used as the metric namespace
    metric-prefix: "" 
    # a boolean, if true the subscription name will be appended to the metric name after the prefix
    append-subscription-name: false 
    # boolean, if true the message timestamp is changed to current time
    override-timestamps: false
    # a boolean, enables exporting timestamps received from the gNMI target as part of the metrics
    export-timestamps: false 
    # a boolean, enables setting string type values as prometheus metric labels.
    strings-as-labels: false
    # a boolean, if set to true, the received gNMI notifications are stored in a cache.
    # the prometheus metrics are generated at the time a prometheus server sends scrape request.
    # this behavior allows the processors (if defined) to be run on all the generated events at once.
    # this mode uses more resource compared to the default one, but offers more flexibility when it comes 
    # to manipulating the data to customize the returned metrics using event-processors.
    gnmi-cache: false
    # duration, scrape request timeout.
    # this timer is started when a scrape request is received, 
    # if it is reached, the metrics generation/collection is stopped.
    timeout: 10s
    # enable debug for prometheus output
    debug: false 
    # string, one of `overwrite`, `if-not-present`, ``
    # This field allows populating/changing the value of Prefix.Target in the received message.
    # if set to ``, nothing changes 
    # if set to `overwrite`, the target value is overwritten using the template configured under `target-template`
    # if set to `if-not-present`, the target value is populated only if it is empty, still using the `target-template`
    add-target: 
    # string, a GoTemplate that allow for the customization of the target field in Prefix.Target.
    # it applies only if the previous field `add-target` is not empty.
    # if left empty, it defaults to:
    # {{- if index . "subscription-target" -}}
    # {{ index . "subscription-target" }}
    # {{- else -}}
    # {{ index . "source" | host }}
    # {{- end -}}`
    # which will set the target to the value configured under `subscription.$subscription-name.target` if any,
    # otherwise it will set it to the target name stripped of the port number (if present)
    target-template:
    # list of processors to apply on the message before writing
    event-processors: 
    # Enables Consul service registration
    service-registration:
      # Consul server address, default to localhost:8500
      address:
      # Consul Data center, defaults to dc1
      datacenter: 
      # Consul username, to be used as part of HTTP basicAuth
      username:
      # Consul password, to be used as part of HTTP basicAuth
      password:
      # Consul Token, is used to provide a per-request ACL token which overrides the agent's default token
      token:
      # Prometheus service check interval, for both http and TTL Consul checks,
      # defaults to 5s
      check-interval:
      # Maximum number of failed checks before the service is deleted by Consul
      # defaults to 3
      max-fail:
      # Consul service name
      name:
      # List of tags to be added to the service registration, 
      # if available, the instance-name and cluster-name will be added as tags,
      # in the format: gnmic-instance=$instance-name and gnmic-cluster=$cluster-name
      tags:
      # bool, enables http service check on top of the TTL check
      enable-http-check:
      # string, if enable-http-check is true, this field can be used to specify the http endpoint to be used to the check
      # if provided, this filed with be prepended with 'http://' (if not already present), 
      # and appended with the value in 'path' field.
      # if not specified, it will be derived from the fields 'listen' and 'path'
      http-check-address:
      # if set to true, the gnmic instance will try to ac quire a lock before registering the prometheus output in consul.
      # this allows to register a single instance of the cluster in consul.
      # if the instance which acquired the lock fails, one of the remaining ones will take over.
      use-lock: false
```

`gnmic` creates the prometheus metric name and its labels from the subscription name, the gnmic path and the value name.

## Metric Generation

The below diagram shows an example of a prometheus metric generation from a gnmi update

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/prometheus_transformation.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fprometheus_transformation.drawio" async></script>

### Metric Naming

The metric name starts with the string configured under __metric-prefix__. 

Then if __append-subscription-name__ is `true`, the __subscription-name__ as specified in `gnmic` configuration file is appended.

The resulting string is followed by the gNMI __path__ stripped of its keys if there are any.

All non-alphanumeric characters are replaced with an underscore "`_`"

The 3 strings are then joined with an underscore "`_`"

If further customization of the metric name is required, the [processors](../event_processors/intro.md) can be used to transform the metric name.

For example, a gNMI update from subscription `port-stats` with path:

```bash
/interfaces/interface[name=1/1/1]/subinterfaces/subinterface[index=0]/state/counters/in-octets
```

is exposed as a metric named:

```bash
gnmic_port_stats_interfaces_interface_subinterfaces_subinterface_state_counters_in_octets
```

### Metric Labels

The metrics labels are generated from the subscription metadata (e.g: `subscription-name` and `source`) and the keys present in the gNMI path elements.

For the previous example the labels would be:

```bash
{interface_name="1/1/1",subinterface_index=0,source="$routerIP:Port",subscription_name="port-stats"}
```

## Service Registration

`gnmic` supports `prometheus_output` service registration via `Consul`.

It allows `prometheus` to dynamically discover new instances of `gnmic` exposing a prometheus server ready for scraping via its [service discovery feature](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config).

If the configuration section `service-registration` is present under the output definition, `gnmic` will register the `prometheus_output` service in `Consul`.

### Configuration Example

The below configuration, registers a service name `gnmic-prom-srv` with `IP=10.1.1.1` and `port=9804`

```yaml
# gnmic.yaml
outputs:
  output1:
    type: prometheus
    listen: 10.1.1.1:9804
    path: /metrics 
    service-registration:
      address: consul-agent.local:8500
      name: gnmic-prom-srv
```

This allows running multiple instances of `gnmic` with minimal configuration changes by using `prometheus` [service discovery feature](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config).

Simplified scrape configuration in the prometheus client:

```yaml
# prometheus.yaml
scrape_configs:
  - job_name: 'gnmic'
    scrape_interval: 10s
    consul_sd_configs:
      - server: $CONSUL_ADDRESS
        services:
          - $service_name
```

### Service Name and ID

The `$service_name` to be discovered by `prometheus` is configured under `outputs.$output_name.service-registration.name`.

If the service registration name field is not present, the name `prometheus-${output_name}` will be used.

In both cases the service ID will be `prometheus-${output_name}-${instance_name}`.

### Service Checks

`gnmic` registers the service in `Consul` with a `ttl` check enabled by default:

* `ttl`: `gnmic` periodically updates the service definition in `Consul`. The goal is to allow `Consul` to detect a same instance restarting with a different service name.

If `service-registration.enable-http-check` is `true`, an `http` check is added:

* `http`: `Consul` periodically scrapes the prometheus server endpoint to check its availability.

```yaml
# gnmic.yaml
outputs:
  output1:
    type: prometheus
    listen: 10.1.1.1:9804
    path: /metrics 
    service-registration:
      address: consul-agent.local:8500
      name: gnmic-prom-srv
      enable-http-check: true
```

Note that for the `http` check to work properly, a reachable address ( IP or name ) should be specified under `listen`.

Otherwise, a reachable address should be added under `service-registration.http-check-address`.
