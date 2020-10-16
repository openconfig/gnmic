`gnmic` supports exposing gnmi updates on a prometheus server, for a prometheus client to scrape.

A Prometheus output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  group1:
    - type: prometheus # required
      listen: :9804 # address to listen on for incoming scape requests
      path: /metrics # path to query to get the metrics
      expiration: 60s # maximum lifetime of metrics in the local cache
      debug: false # enable debug for prometheus output
```

`gnmic` creates the prometheus metric name and its labels from the subscription name, the gnmic path and the value name.

### Metric Naming

The metric name starts with the subscription name, followed by the path stripped from keys if there are any, then the value name. The 3 sections are joined with an underscore "`_`"

Characters "`/`", "`:`" and "`-`" are replaced with a "`_`".

For example, a gnmi update from subscription `port-stats` with path:

```bash
/interfaces/interface[name=1/1/1]/subinterfaces/subinterface[index=0]/state/counters/in-octets
```

is exposed as a metric named: 
```bash
port_stats_interfaces_interface_subinterfaces_subinterface_state_counters_in_octets
```

### Metric Labels
The metrics labels are generated from the subscripion metadata (e.g: `subscription-name` and `source`) and the keys present in the gnmi path elements.

For the previous example the labels would be: 

```bash
{interface_name="1/1/1",subinterface_index=0,source="$routerIP:Port",subscription_name="port-stats"}
```
