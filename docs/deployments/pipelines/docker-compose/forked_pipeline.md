
The purpose of this deployment is to create a forked data pipeline using `NATS` , `Influxdb` and `Prometheus`

The example includes 3 `gnmic` instances.

- The first, called `collector`, is responsible for streaming the gNMI data from the targets and output it to a `NATS` server.
- The second and third, called `relay1` and `relay2`, reads the data from `NATS` and writes it to either `InfluxDB` or `Prometheus`

This deployment enables a few use cases:

- Apply different [processors](../../../user_guide/event_processors/intro.md) by the collector and relay.
- Scale the collector and relay separately, see this [example](gnmic_cluster_nats_prometheus.md) for a scaled-out version.
- Fork the data into a separate pipeline for a different use case.


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/pipeline_gnmic_nats_gnmic_prometheus_gnmic_influxdb.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fpipeline_gnmic_nats_gnmic_prometheus_gnmic_influxdb.drawio" async></script>

Deployment files:

- [docker compose](https://github.com/karimra/gnmic/blob/main/examples/deployments/docker-compose/3.pipelines/4.gnmic-nats-gnmic-prometheus-gnmic-influxdb/docker-compose.yaml)

- [gnmic collector config](https://github.com/karimra/gnmic/blob/main/examples/deployments/docker-compose/3.pipelines/4.gnmic-nats-gnmic-prometheus-gnmic-influxdb/gnmic-collector.yaml)
- [gnmic relay1 config](https://github.com/karimra/gnmic/blob/main/examples/deployments/docker-compose/3.pipelines/4.gnmic-nats-gnmic-prometheus-gnmic-influxdb/gnmic-relay1.yaml)
- [gnmic relay2 config](https://github.com/karimra/gnmic/blob/main/examples/deployments/docker-compose/3.pipelines/4.gnmic-nats-gnmic-prometheus-gnmic-influxdb/gnmic-relay2.yaml)
- [prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/docker-compose/3.pipelines//4.gnmic-nats-gnmic-prometheus-gnmic-influxdb/prometheus/prometheus.yaml)

Download the files, update the `gnmic` collector config files with the desired subscriptions and targets.

Deploy it with:

```bash
sudo docker-compose up -d
```

Check the [Prometheus Output](../../../user_guide/outputs/prometheus_output.md) and [NATS Input](../../../user_guide/inputs/nats_input.md) documentation page for more configuration options
