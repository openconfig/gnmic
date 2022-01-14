
The purpose of this deployment is to create data pipeline using `NATS` and `Prometheus`

The example includes 2 `gnmic` instances.

- The first, called `collector`, is responsible for streaming the gNMI data from the targets and output it to a `NATS` server.
- The second, called `relay`, reads the data from `NATS` and writes it to `Prometheus`

This deployment enables a few use cases:

- Apply different [processors](../../../user_guide/event_processors/intro.md) by the collector and relay.
- Scale the collector and relay separately, see this [example](gnmic_cluster_nats_prometheus.md) for a scaled-out version.
- Fork the data into a separate pipeline for a different use case.



<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/pipeline_nats_prometheus.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fpipeline_nats_prometheus.drawio" async></script>

Deployment files:

- [docker compose](https://github.com/karimra/gnmic/blob/main/examples/deployments/3.pipelines/1.gnmic-nats-gnmic-prometheus/docker-compose/docker-compose.yaml)

- [gnmic collector config](https://github.com/karimra/gnmic/blob/main/examples/deployments/3.pipelines/1.gnmic-nats-gnmic-prometheus/docker-compose/gnmic-collector.yaml)
- [gnmic relay config](https://github.com/karimra/gnmic/blob/main/examples/deployments/3.pipelines/1.gnmic-nats-gnmic-prometheus/docker-compose/gnmic-relay.yaml)

Download the files, update the `gnmic` collector config files with the desired subscriptions and targets.

Deploy it with:

```bash
sudo docker-compose up -d
```

Check the [Prometheus Output](../../../user_guide/outputs/prometheus_output.md) and [NATS Input](../../../user_guide/inputs/nats_input.md) documentation page for more configuration options
