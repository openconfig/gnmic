`gnmic` supports writing metrics to Prometheus using its [remote write API](https://grafana.com/blog/2019/03/25/whats-new-in-prometheus-2.8-wal-based-remote-write/).

`gNMIc`'s prometheus remote write can be used to push metrics to a variety of monitoring systems like [Mimir](https://grafana.com/oss/mimir/), [CortexMetrics](https://cortexmetrics.io/), [VictoriaMetrics](https://victoriametrics.com/), [Thanos](https://thanos.io/)...

A Prometheus write output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    # required
    type: prometheus_write
    # url to push metrics towards, scheme is required
    url: http://<grafana-mimir-addr>:9009/api/v1/push
    # a map of string:string, 
    # custom HTTP headers to be sent along with each remote write request.
    headers:
    # sets the `Authorization` header on every remote write request with the
    # configured username and password.
    authentication:
      username:
      password:
    # sets the `Authorization` header with type `.authorization.type` and the token value.
    authorization:
      type: Bearer
      credentials: <token string>
    # TLS configuration
    tls:
      # string, path to CA certificates file
      ca-file:
      # string, path to client certificate file
      cert-file:
      # string, path to client key file
      key-file:
      # boolean, if true, the client does not verify the server certificates
      skip-verify:
    # duration, defaults to 10s, time interval between write requests
    interval: 10s
    # integer, defaults to 1000.
    # Buffer size for time series to be sent to the remote system.
    # metrics are sent to the remote system every `.interval` or when the buffer is full. Whichever one is reached first.
    buffer-size: 1000
    # integer, defaults to 500, sets the maximum number of timeSeries per write request to remote.
    max-time-series-per-write: 500
    # integer, defaults to 0
    # number of retries per write, retries will have a back off of 100ms.
    max-retries: 0
    # metadata configuration
    metadata:
      # boolean, 
      # if true, metrics metadata is sent.
      include: false
      # duration, defaults to 60s.
      # Applies if `metadata.include` is set to true
      # Interval after which all metadata entries are sent to the remote write address
      interval: 60s
      # integer, defaults to 500
      # applies if `metadata.include` is set to true
      # Max number of metadata entries per write request.
      max-entries-per-write: 500
    # string, to be used as the metric namespace
    metric-prefix: "" 
    # boolean, if true the subscription name will be appended to the metric name after the prefix
    append-subscription-name: false 
    # boolean, enables setting string type values as prometheus metric labels.
    strings-as-labels: false
    # duration, defaults to 10s
    # Push request timeout.
    timeout: 10s
    # boolean, defaults to false
    # Enables debug for prometheus write output.
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
