`gnmic` supports exporting subscription updates to [influxDB](https://www.influxdata.com/products/influxdb-overview/) time series database

An influxdb output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  group1:
    - type: influxdb # required
      url: http://localhost:8086 # influxDB server address
      org: myOrg # empty if using influxdb1.8.x
      bucket: telemetry # string in the form database/retention-policy. Skip retention policy for the default on
      token: # influxdb 1.8.x use a string in the form: "username:password"
      batch_size: 1000 # number of points to buffer before writing to the server
      flush_timer: 10s # flush period after which the buffer is written to the server whether the batch_size is reached or not
      use_gzip: false
      enable_tls: false
      health_check_period: 30s # server health check period, used to recover from server connectivity failure
      debug: false # enable debug
```

`gnmic` uses the [`event`](../output_intro#formats-examples) format to generate the measurements written to influxdb
