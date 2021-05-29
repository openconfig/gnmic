`gnmic` supports exporting subscription updates to [influxDB](https://www.influxdata.com/products/influxdb-overview/) time series database

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
    use-gzip: false
    enable-tls: false
    # boolean, if true the message timestamp is changed to current time
    override-timestamps: false 
    # server health check period, used to recover from server connectivity failure
    health-check-period: 30s 
    # enable debug
    debug: false 
    # NOT IMPLEMENTED boolean, enables the collection and export (via prometheus) of output specific metrics
    enable-metrics: false 
    # list of processors to apply on the message before writing
    event-processors: 
```

`gnmic` uses the [`event`](../output_intro#formats-examples) format to generate the measurements written to influxdb
