`gnmic` supports exporting subscription updates to [influxDB](https://www.influxdata.com/products/influxdb-overview/) time series database

## Configuration

An influxdb output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    # required
    type: influxdb 
    # influxDB server address
    url: http://localhost:8086 
    # empty if using influxdb1.8.x
    org: myOrg 
    # string in the form database/retention-policy. Skip retention policy for the default on
    bucket: telemetry
    # influxdb 1.8.x use a string in the form: "username:password"
    token: 
    # number of points to buffer before writing to the server
    batch-size: 1000 
    # flush period after which the buffer is written to the server whether the batch_size is reached or not
    flush-timer: 10s
    # if true, the influxdb client will use gzip compression in write requests.
    use-gzip: false
    # if true, the influxdb client will use a secure connection to the server.
    enable-tls: false
    # boolean, if true the message timestamp is changed to current time
    override-timestamps: false 
    # server health check period, used to recover from server connectivity failure
    health-check-period: 30s 
    # enable debug
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
    # NOT IMPLEMENTED boolean, enables the collection and export (via prometheus) of output specific metrics
    enable-metrics: false 
    # list of processors to apply on the message before writing
    event-processors: []
    # cache, if present enables the influxdb output to cache received updates and write them all together 
    # at `cache-flush-timer` expiry.
    cache:
      # duration, if > 0, enables the expiry of values written to the cache.
      expiration: 0s
      # debug, if true enable extra logging
      debug: false
    # cache-flush-timer
    cache-flush-timer: 5s
```

`gnmic` uses the [`event`](../event_processors/intro.md#the-event-format) format to generate the measurements written to influxdb.

## Caching

When caching is enabled, the received messages are not written directly to InfluxDB, they are first cached as gNMI updates and written in batch when the `cache-flush-timer` is reached.

The below diagram shows how an InfluxDB output works with and without cache enabled:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:10,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/influxdb_output_with_without_cache.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2F/influxdb_output_with_without_cache.drawio" async></script>

When caching is enabled, the cached gNMI updates are periodically retrieved in batch, converted to [events](../event_processors/intro.md#the-event-format).
If [processors](../event_processors/intro.md) are defined under the output, they are applied to the whole list of events at once. This allows augmenting some messages with values from other messages even if they where collected from a different target/subscription.
