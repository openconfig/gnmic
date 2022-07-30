In the context of gnmi subscriptions (on top of terminal output) `gnmic` supports multiple output options:

* [Local file](file_output.md)
* [NATS messaging system](nats_output.md)
* [NATS Streaming messaging bus (STAN)](stan_output.md)
* [NATS JetStream](jetstream_output.md)
* [Kafka messaging bus](kafka_output.md)
* [InfluxDB Time Series Database](influxdb_output.md)
* [Prometheus Server](prometheus_output.md)
* [Prometheus Remote Write](prometheus_write_output.md)
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

### Caching

By default, `gNMIc` outputs write the received gNMI updates as they arrive (i.e without caching).

Caching messages before writing them to a remote location can yield a few benefits like **rate limiting**, **batch processing**, **data replication**, etc.

Both `influxdb` and `prometheus` outputs support caching messages before exporting.
Caching support by other outputs is planned.

#### How does it work?

When caching is enabled for a certain output, the received messages are not written directly to the output remote server (for e.g: InfluxDB server), but rather cached locally until the `cache-flush-timer` is reached (in the case of an `influxdb` output) or when the output receives a `Prometheus` scrape request (in the case of a `prometheus` output).

The below diagram shows how an InfluxDB output works with and without cache enabled:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:10,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/influxdb_output_with_without_cache.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2F/influxdb_output_with_without_cache.drawio" async></script>

When caching is enabled, the cached gNMI updates are periodically retrieved from the cache in batch, converted to [events](../event_processors/intro.md#the-event-format).
If [processors](../event_processors/intro.md) are defined under the output, they are applied to the whole list of events at once. This allows augmenting some messages with values from other messages even if they where collected from a different target/subscription.

#### Enable caching

Caching can be enabled per output by adding the following configuration snippet under the desired output:

```yaml
outputs:
  output1:
    type: prometheus
    #
    # other output related fields
    #
    cache: {}
```

This enables `output1` to use a cache of type [`oc`](#gnmi-cache).

Each output has its own cache.
Using a single global cache will be implemented in a future release.

#### Distributed caches

When running multiple instances of `gNMIc` it's possible to synchronize the collected data between all the instances using a distributed cache.

Each output configured with a remote cache will write the collected gNMI update to the cache first, then read back all written data to process it and eventually write it to the output server.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:10,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/distributed_caches.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2F/distributed_caches.drawio" async></script>

This is useful when different instances collect data from different targets and/or subscriptions. A single instance can be responsible for writing all the collected data to the output or each instance would be writing to a different output.

#### Cache types

`gNMIc` supports 4 cache types. The choice of cache to use depends on the use case you are trying to achieve.

##### gNMI cache

Is an in-memory gNMI cache based on the Openconfig gNMI cache published [here](https://github.com/openconfig/gnmi/tree/master/cache)

This type of cache is ideal when running a single `gNMIc` instance. It is also the default cache type when caching is enabled.

Configuration:

```yaml
outputs:
  output1:
    type: prometheus # or influxdb
    #
    # other output related fields
    #
    cache: 
      type: oc
      # duration, default: 60s.
      # updates older than the expiration value will not be read from the cache.
      expiration: 60s
      # enable extra logging
      debug: false
```

##### NATS cache

Is a cache type that relies on a [NATS server](https://docs.nats.io/) to distribute the collected updates between `gNMIc` instances.

This type of cache is useful when multiple `gNMIc` instances are subscribed to different targets and/or different gNMI paths.

Configuration:

```yaml
outputs:
  output1:
    type: prometheus # or influxdb
    #
    # other output related fields
    #
    cache:
      type: nats
      # string, address of the remote NATS server,
      # if left empty an in memory NATS server will be created an used.
      address:
      # string, the NATS server username.
      username:
      # string, the NATS server password.
      password:
      # string, expiration period of received messages.
      expiration: 60s
      # enable extra logging
      debug: false
```

##### JetStream cache

Is a cache type that relies on a [JetStream server](https://docs.nats.io/nats-concepts/jetstream) to distribute the collected updates between `gNMIc` instances.

This type of cache is useful when multiple `gNMIc` instances are subscribed to different targets and/or different gNMI paths.

It is planned to add [gNMI historical subscriptions](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-history.md#1-purpose) support using the `jetstream` cache type.

Configuration:

```yaml
outputs:
  output1:
    type: prometheus # or influxdb
    #
    # other output related fields
    #
    cache:
      type: jetstream
      # string, address of the remote NATS JetStream server,
      # if left empty an in memory NATS JetStream server will be created an used.
      address:
      # string, the JetStream server username.
      username:
      # string, the JetStream server password.
      password:
      # duration, default: 60s.
      # Expiration period of received messages.
      expiration: 60s
      # int64, default: 1073741824 (1 GiB). 
      # Max number of bytes stored in the cache per subscription.
      max-bytes:
      # int64, default: 1048576. 
      # Max number of messages stored per subscription.
      max-msgs-per-subscription:
      # int, default 100. 
      # Batch size used by the JetStream pull subscriber.
      fetch-batch-size:
      # duration, default 100ms. 
      # Wait time used by the JetStream pull subscriber.
      fetch-wait-time:
      # enable extra logging
      debug: false      
```

##### Redis cache

Is a cache type that relies on a [Redis PUBSUB server](https://redis.io/docs/manual/pubsub/) to distribute the collected updates between `gNMIc` instances.

This type of cache is useful when multiple `gNMIc` instances are subscribed to different targets and/or different gNMI paths.

```yaml
outputs:
  output1:
    type: prometheus # or influxdb
    #
    # other output related fields
    #
    cache:
      type: redis
      # string, redis server address
      address:
      # string, the Redis server username.
      username:
      # string, the Redis server password.
      password:
      # duration, default: 60s.
      # Expiration period of received messages.
      expiration: 60s
      # enable extra logging
      debug: false
```
