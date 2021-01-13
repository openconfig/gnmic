`gnmic` supports exporting subscription updates to a TCP server

A TCP output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    type: tcp # required
    address: IPAddress:Port # a UDP server address 
    rate: 10ms # maximum sending rate, e.g: 1ns, 10ms
    buffer-size: # number of messages to buffer in case of sending failure
    format: json # export format. json, protobuf, prototext, protojson, event
    keep-alive: # enable TCP keepalive and specify the timer, e.g: 1s, 30s
    retry-interval: # time duration to wait before re-dial in case there is a failure
    enable-metrics: false # NOT IMPLEMENTED boolean, enables the collection and export (via prometheus) of output specific metrics
    event-processors: # list of processors to apply on the mesage before writing
```

A TCP output can be used to export data to an ELK stack, using [Logstash TCP input](https://www.elastic.co/guide/en/logstash/current/plugins-inputs-tcp.html)