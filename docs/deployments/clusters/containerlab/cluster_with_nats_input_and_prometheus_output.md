The purpose of this deployment is to achieve __redundancy__, __high-availability__ as well as __data replication__.

The redundancy and high-availability are guaranteed by deploying a `gnmic` cluster.

The data replication is achieved using a `NATS` server acting as both a gnmic input and output.

This deployment example includes a:

- 3 instances [gNMIc cluster](../../../user_guide/HA.md), 
- A [NATS](https://nats.io/) Server acting as both [input](../../../user_guide/inputs/nats_input.md) and [output](../../../user_guide/outputs/nats_output.md) 
- A [Prometheus](https://prometheus.io/) Server
- A [Grafana](https://grafana.com/docs/) Server
- A [Consul](https://www.consul.io/docs/intro) Server

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


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/cluster_clab_prom_nats.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fcluster_clab_prom_nats.drawio" async></script>

Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/3.nats-input-prometheus-output/containerlab/lab23.clab.yaml)
- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/3.nats-input-prometheus-output/containerlab/gnmic.yaml)
- [prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/3.nats-input-prometheus-output/containerlab/prometheus/prometheus.yaml)
- [Grafana datasource](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/3.nats-input-prometheus-output/containerlab/grafana/datasources/datasource.yaml)

Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/2.clusters/3.nats-input-prometheus-output/containerlab
sudo clab deploy -t lab23.clab.yaml
```

```text
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
| #  |          Name           | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address   |     IPv6 Address      |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
|  1 | clab-lab23-consul-agent | cfdaf19e9435 | consul:latest                | linux |       | running | 172.20.20.8/24  | 2001:172:20:20::8/64  |
|  2 | clab-lab23-gnmic1       | 7e2a4060a1ae | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.3/24  | 2001:172:20:20::3/64  |
|  3 | clab-lab23-gnmic2       | 9e27e4620104 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.4/24  | 2001:172:20:20::4/64  |
|  4 | clab-lab23-gnmic3       | bb7471eb5f49 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.5/24  | 2001:172:20:20::5/64  |
|  5 | clab-lab23-grafana      | 3fbf7755c49e | grafana/grafana:latest       | linux |       | running | 172.20.20.2/24  | 2001:172:20:20::2/64  |
|  6 | clab-lab23-leaf1        | a61624d5312b | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.21/24 | 2001:172:20:20::15/64 |
|  7 | clab-lab23-leaf2        | ef86f701b379 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.14/24 | 2001:172:20:20::e/64  |
|  8 | clab-lab23-leaf3        | 352433a2ab3b | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.22/24 | 2001:172:20:20::16/64 |
|  9 | clab-lab23-leaf4        | 5ddba813d36f | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.19/24 | 2001:172:20:20::13/64 |
| 10 | clab-lab23-leaf5        | aad20f4b9969 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.11/24 | 2001:172:20:20::b/64  |
| 11 | clab-lab23-leaf6        | 757c76527a75 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.15/24 | 2001:172:20:20::f/64  |
| 12 | clab-lab23-leaf7        | d85e94aaa0dd | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.10/24 | 2001:172:20:20::a/64  |
| 13 | clab-lab23-leaf8        | ef6210c0e5aa | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.20/24 | 2001:172:20:20::14/64 |
| 14 | clab-lab23-nats         | f1a1f351bbf8 | nats:latest                  | linux |       | running | 172.20.20.6/24  | 2001:172:20:20::6/64  |
| 15 | clab-lab23-prometheus   | f7f194a934c5 | prom/prometheus:latest       | linux |       | running | 172.20.20.7/24  | 2001:172:20:20::7/64  |
| 16 | clab-lab23-spine1       | ddbf4e804097 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.16/24 | 2001:172:20:20::10/64 |
| 17 | clab-lab23-spine2       | f48323a4de88 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.17/24 | 2001:172:20:20::11/64 |
| 18 | clab-lab23-spine3       | 2a65eed26a7e | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.18/24 | 2001:172:20:20::12/64 |
| 19 | clab-lab23-spine4       | ea59d0e5d9ed | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.12/24 | 2001:172:20:20::c/64  |
| 20 | clab-lab23-super-spine1 | 37af6cd04dd8 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.9/24  | 2001:172:20:20::9/64  |
| 21 | clab-lab23-super-spine2 | 3408891a0718 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.13/24 | 2001:172:20:20::d/64  |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
```
Check the  [NATS Output](../../../user_guide/outputs/nats_output.md), [NATS Input](../../../user_guide/inputs/nats_input.md) and  [Prometheus Output](../../../user_guide/outputs/influxdb_output.md) documentation pages for more configuration options.
