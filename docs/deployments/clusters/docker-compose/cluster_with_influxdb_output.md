The purpose of this deployment is to achieve __redundancy__, __high-availability__ via clustering.

This deployment example includes:

- A 3 instances [`gnmic` cluster](../../../user_guide/HA.md),
- A single [InfluxDB output](../../../user_guide/outputs/influxdb_output.md)

The leader election and target distribution is done with the help of a [Consul server](https://www.consul.io/docs/introhttps://www.consul.io/docs/intro)

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/cluster_influxdb.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fcluster_influxdb.drawio" async></script>

Deployment files:

- [Docker Compose](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/1.influxdb-output/docker-compose/docker-compose.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/1.influxdb-output/docker-compose/gnmic.yaml)

Download the files, update the `gnmic` config files with the desired subscriptions and targets.

Deploy it with:

```bash
sudo docker-compose up -d
```

Check the [InfluxDB Output](../../../user_guide/outputs/influxdb_output.md) documentation page for more configuration options.
