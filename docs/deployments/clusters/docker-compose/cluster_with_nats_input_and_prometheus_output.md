The purpose of this deployment is to achieve __redundancy__, __high-availability__ as well as __data replication__.

The redundancy and high-availability are guaranteed by deploying a `gnmic` cluster.

The data replication is achieved using a `NATS` server acting as both a gnmic input and output.

This deployment example includes a:

- 3 instances [`gnmic` cluster](../../../user_guide/HA.md), 
- A NATS [input](../../../user_guide/inputs/nats_input.md) and [output](../../../user_guide/outputs/nats_output.md) 
- A [Prometheus output](../../../user_guide/outputs/prometheus_output.md)

The leader election and target distribution is done with the help of a [Consul server](https://www.consul.io/docs/introhttps://www.consul.io/docs/intro)

Each `gnmic` instance outputs the streamed gNMI data to NATS, and reads back all the data from the same NATS server (including its own),

This effectively guarantees that each instance holds the data streamed by the whole cluster.

Like in the previous examples, each `gnmic` instance will also register its Prometheus output service in `Consul`.

But before doing so, it will attempt to acquire a key lock `gnmic/$CLUSTER_NAME/prometheus-output`,  (`use-lock: true`)

```yaml
prom-output:
  type: prometheus
  listen: ":9804"
  service-registration:
    address: consul-agent:8500
    use-lock: true # <===
```

Since only one instance can hold a lock, only one prometheus output is registered, so only one output is scraped by Prometheus.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/cluster_nats_prometheus.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fcluster_nats_prometheus.drawio" async></script>

Deployment files:

- [Docker Compose](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/3.nats-input-prometheus-output/docker-compose/docker-compose.yaml)
- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/3.nats-input-prometheus-output/docker-compose/gnmic.yaml)
- [Prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/3.nats-input-prometheus-output/docker-compose/prometheus/prometheus.yaml)

Download the files, update the `gnmic` config files with the desired subscriptions and targets.

!!! note
    The targets outputs list should include the nats output name

Deploy it with:

```bash
sudo docker-compose up -d
```

Check the  [NATS Output](../../../user_guide/outputs/nats_output.md), [NATS Input](../../../user_guide/inputs/nats_input.md) and  [Prometheus Output](../../../user_guide/outputs/influxdb_output.md) documentation pages for more configuration options.
