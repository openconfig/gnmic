`gnmic` supports exporting subscription updates to a TCP server

A TCP output can be defined using the below format in `gnmic` config file under `outputs` section:

```yaml
outputs:
  output1:
    # required
    type: tcp 
    # a UDP server address 
    address: IPAddress:Port 
    # maximum sending rate, e.g: 1ns, 10ms
    rate: 10ms 
    # number of messages to buffer in case of sending failure
    buffer-size:
    # export format. json, protobuf, prototext, protojson, event
    format: json 
    # boolean, if true the message timestamp is changed to current time
    override-timestamps: false
    # enable TCP keepalive and specify the timer, e.g: 1s, 30s
    keep-alive: 
    # time duration to wait before re-dial in case there is a failure
    retry-interval: 
    # NOT IMPLEMENTED boolean, enables the collection and export (via prometheus) of output specific metricss
    enable-metrics: false 
    # list of processors to apply on the message before writing
    event-processors: 
```

A TCP output can be used to export data to an ELK stack, using [Logstash TCP input](https://www.elastic.co/guide/en/logstash/current/plugins-inputs-tcp.html)