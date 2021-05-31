In the context of gnmi subscriptions (on top of terminal output) `gnmic` supports multiple output options:

* [Local file](file_output.md)
* [NATS messaging system](nats_output.md)
* [NATS Streaming messaging bus (STAN)](stan_output.md)
* [Kafka messaging bus](kafka_output.md)
* [InfluxDB Time Series Database](influxdb_output.md)
* [Prometheus Server](prometheus_output.md)
* [UDP Server](udp_output.md)
* [TCP Server](tcp_output.md)

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/outputs.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2F/outputs.drawio" async></script>

These outputs can be mixed and matched at will with the different gnmi subscribe targets.

With multiple outputs defined in the [configuration file](../configuration_file.md) you can collect once 
and export the subscriptions updates to multiple locations formatted differently.

### Defining outputs

To define an output a user needs to create the `outputs` section in the configuration file:

```yaml
# part of ~/gnmic.yml config file
outputs:
  output1:
    type: file # output type
    file-type: stdout # or stderr
    format: json
  output2:
    type: file
    filename: /path/to/localFile.log  
    format: protojson
  output3:
    type: nats # output type
    address: 127.0.0.1:4222 # comma separated nats servers addresses
    subject-prefix: telemetry #
    format: event
  output4:
    type: file
    filename: /path/to/localFile.log  
    format: json
  output5:
    type: stan # output type
    address: 127.0.0.1:4223 # comma separated nats streaming servers addresses
    subject: telemetry #
    cluster-name: test-cluster #
    format: proto
  output6:
    type: kafka # output type
    address: localhost:9092 # comma separated kafka brokers addresses
    topic: telemetry # kafka topic
    format: proto
  output7:
    type: stan # output type
    address: 127.0.0.1:4223 # comma separated nats streaming servers addresses
    subject: telemetry
    cluster-name: test-cluster
```

!!! note
    Outputs names are case insensitive

#### Output formats

Different formats are supported for all outputs

**Format/output** | **proto**                          | **protojson**                   |  **prototext**                      | **json**                       | **event**
----------------- | ---------------------------------- | --------------------------------| ------------------------------------|--------------------------------|--------------------------------:
**File**          | <span style="color:red">:x:</span> | <span>:heavy_check_mark:</span> | <span>:heavy_check_mark:</span>     |<span>:heavy_check_mark:</span> |<span>:heavy_check_mark:</span>
**NATS / STAN**   | <span>:heavy_check_mark:</span>    | <span>:heavy_check_mark:</span> | <span style="color:red">:x: </span> |<span>:heavy_check_mark:</span> |<span>:heavy_check_mark:</span>
**Kafka**         | <span>:heavy_check_mark:</span>    | <span>:heavy_check_mark:</span> | <span style="color:red">:x: </span> |<span>:heavy_check_mark:</span> |<span>:heavy_check_mark:</span>
**UDP / TCP**     | <span>:heavy_check_mark:</span>    | <span>:heavy_check_mark:</span> | <span>:heavy_check_mark:</span>     |<span>:heavy_check_mark:</span> |<span>:heavy_check_mark:</span>
**InfluxDB**      | <span>NA</span>                    | <span>NA</span>                 | <span>NA</span>                     |<span>NA</span>                 |<span>NA</span>                    
**Prometheus**    | <span>NA</span>                    | <span>NA</span>                 | <span>NA</span>                     |<span>NA</span>                 |<span>NA</span>                    

#### Formats examples

=== "protojson"
    ```json
    {
      "update": {
      "timestamp": "1595491618677407414",
      "prefix": {
        "elem": [
          {
            "name": "configure"
          },
          {
            "name": "system"
          }
        ]
      },
      "update": [
        {
          "path": {
            "elem": [
              {
                "name": "name"
              }
            ]
            },
            "val": {
              "stringVal": "sr123"
            }
          }
        ]
      }
    }
    ```
=== "prototext"
    ```yaml
    update: {
      timestamp: 1595491704850352047
      prefix: {
        elem: {
          name: "configure"
        }
        elem: {
          name: "system"
        }
      }
      update: {
        path: {
          elem: {
            name: "name"
          }
        }
        val: {
          string_val: "sr123"
        }
      }
    }
    ```
=== "json"
    ```json
    {
      "source": "172.17.0.100:57400",
      "subscription-name": "sub1",
      "timestamp": 1595491557144228652,
      "time": "2020-07-23T16:05:57.144228652+08:00",
      "prefix": "configure/system",
      "updates": [
        {
          "Path": "name",
          "values": {
            "name": "sr123"
          }
        }
      ]
    }
    ```
=== "event"
    ```json
    [
      {
        "name": "sub1",
        "timestamp": 1595491586073072000,
        "tags": {
          "source": "172.17.0.100:57400",
          "subscription-name": "sub1"
      },
        "values": {
          "/configure/system/name": "sr123"
        }
      }
    ]
    ```

### Binding outputs

Once the outputs are defined, they can be flexibly associated with the targets.

```yaml
# part of ~/gnmic.yml config file
targets:
  router1.lab.com:
    username: admin
    password: secret
    outputs:
      - output1
      - output3
  router2.lab.com:
    username: gnmi
    password: telemetry
    outputs:
      - output2
      - output3
      - output4
```
