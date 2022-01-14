The purpose of this deployment is to achieve __redundancy__, __high-availability__ via clustering.

This deployment example includes:

- A 3 instances [`gnmic` cluster](../../../user_guide/HA.md),
- A single [Prometheus output](../../../user_guide/outputs/prometheus_output.md)

The leader election and target distribution is done with the help of a [Consul server](https://www.consul.io/docs/introhttps://www.consul.io/docs/intro)

`gnmic` will also register its Prometheus output service in `Consul` so that Prometheus can discover which Prometheus servers are available to be scraped

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/cluster_prometheus&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fcluster_prometheus" async></script>

Deployment files:

- [docker compose](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/docker-compose/docker-compose.yaml)
- [gnmic config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/docker-compose/gnmic.yaml)
- [prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/docker-compose/prometheus/prometheus.yaml)

Download the files, update the `gnmic` config files with the desired subscriptions and targets.

Deploy it with:

```bash
sudo docker-compose up -d
```

Check the [Prometheus Output](../../../user_guide/outputs/prometheus_output.md) documentation page for more configuration options.
