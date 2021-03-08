
The purpose of this deployment is to create a clustered data pipeline using `NATS` and `Prometheus`.
Achieving __redundancy__, __high-availability__ and __data replication__, all in clustered data pipeline.

The example is divided in 2 parts:

- Clustered collectors and single relay
- Clustered collectors and clustered relays

These 2 examples are essentially scaled-out versions of this [example](nats_prometheus.md)

### Clustered collectors and single relay

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/pipeline_cluster_nats_prometheus.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fpipeline_cluster_nats_prometheus.drawio" async></script>


Deployment files:

- [docker compose](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3a.gnmic-nats-gnmic-prometheus/docker-compose.yaml)

- [gnmic collector1 config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3a.gnmic-cluster-nats-gnmic-prometheus/gnmic-collector1.yaml)
- [gnmic collector2 config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3a.gnmic-cluster-nats-gnmic-prometheus/gnmic-collector2.yaml)
- [gnmic collector3 config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3a.gnmic-cluster-nats-gnmic-prometheus/gnmic-collector3.yaml)
- [gnmic relay config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3a.gnmic-cluster-nats-gnmic-prometheus/gnmic-relay.yaml)

Download the files, update the `gnmic` collectors config files with the desired subscriptions and targets.

Deploy it with:

```bash
sudo docker-compose up -d
```

Check the [Prometheus Output](../../user_guide/outputs/prometheus_output.md) and [NATS Input](../../user_guide/inputs/nats_input.md) documentation page for more configuration options

### Clustered collectors and clustered relays

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/pipeline_cluster_nats_cluster_prometheus.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fpipeline_cluster_nats_cluster_prometheus.drawio" async></script>



Deployment files:

- [docker compose](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3b.gnmic-cluster-nats-gnmic-cluster-prometheus/docker-compose.yaml)

- [gnmic collector1 config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3b.gnmic-cluster-nats-gnmic-cluster-prometheus/gnmic-collector1.yaml)
- [gnmic collector2 config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3b.gnmic-cluster-nats-gnmic-cluster-prometheus/gnmic-collector2.yaml)
- [gnmic collector3 config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3b.gnmic-cluster-nats-gnmic-cluster-prometheus/gnmic-collector3.yaml)
- [gnmic relay1 config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3b.gnmic-cluster-nats-gnmic-cluster-prometheus/gnmic-relay1.yaml)
- [gnmic relay2 config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3b.gnmic-cluster-nats-gnmic-cluster-prometheus/gnmic-relay2.yaml)
- [gnmic relay3 config](https://github.com/karimra/gnmic/blob/master/examples/deployments/3.pipelines/3b.gnmic-cluster-nats-gnmic-cluster-prometheus/gnmic-relay3.yaml)

Download the files, update the `gnmic` collectors config files with the desired subscriptions and targets.

Deploy it with:

```bash
sudo docker-compose up -d
```

Check the [Prometheus Output](../../user_guide/outputs/prometheus_output.md) and [NATS Input](../../user_guide/inputs/nats_input.md) documentation page for more configuration options