The purpose of this deployment is to achieve __redundancy__, __high-availability__ and __data aggregation__ via clustering.

This deployment example includes:

- A 3 instances [gNMIc cluster](../../../user_guide/HA.md),
- A standalone `gNMIc` instance.
- A [Prometheus](https://prometheus.io/) Server
- A [Grafana](https://grafana.com/docs/) Server
- A [Consul](https://www.consul.io/docs/intro) Server

The leader election and target distribution is done with the help of a [Consul server](https://www.consul.io/docs/introhttps://www.consul.io/docs/intro)

All members of the cluster expose a gNMI Server that the single gNMIc instance will use to aggregate the collected data.

The aggregation `gNMIc` instance exposes a Prometheus output that is registered in `Consul` and is discoverable by the Prometheus server.

The whole lab is pretty much self organising:

- The `gNMIc` cluster instances discover the targets dynamically using a [Docker Loader](../../../user_guide/target_discovery/docker_discovery.md)
- The `gNMIc` standalone instance, discovers the cluster instance using a [Consul Loader](../../../user_guide/target_discovery/consul_discovery.md)
- The Prometheus server discovers gNMIc's Prometheus output using [Consul Service Discovery](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config)

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/clab_cluster_gnmi_server.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fclab_cluster_gnmi_server.drawio" async></script>



Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/4.gnmi-server/containerlab/gnmi-server.clab.yaml)
- [gNMIc cluster config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/4.gnmi-server/containerlab/gnmic.yaml)
- [gNMIc aggregator config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/4.gnmi-server/containerlab/gnmic-agg.yaml)
- [Prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/4.gnmi-server/containerlab/prometheus/prometheus.yaml)
- [Grafana datasource](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/4.gnmi-server/containerlab/grafana/datasources/datasource.yaml)

Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/2.clusters/4.gnmi-server/containerlab
sudo clab deploy -t gnmi-server.clab.yaml
```

```text
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
| #  |          Name           | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address   |     IPv6 Address      |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
|  1 | clab-lab24-agg-gnmic    | 2e9cc2821b07 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.7/24  | 2001:172:20:20::7/64  |
|  2 | clab-lab24-consul-agent | c17d31d5f41b | consul:latest                | linux |       | running | 172.20.20.2/24  | 2001:172:20:20::2/64  |
|  3 | clab-lab24-gnmic1       | 3d56e09955f2 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.4/24  | 2001:172:20:20::4/64  |
|  4 | clab-lab24-gnmic2       | eba24dacea36 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.3/24  | 2001:172:20:20::3/64  |
|  5 | clab-lab24-gnmic3       | caf473f500f6 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.6/24  | 2001:172:20:20::6/64  |
|  6 | clab-lab24-grafana      | eaa224e62243 | grafana/grafana:latest       | linux |       | running | 172.20.20.8/24  | 2001:172:20:20::8/64  |
|  7 | clab-lab24-leaf1        | 6771dc8d3786 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.10/24 | 2001:172:20:20::a/64  |
|  8 | clab-lab24-leaf2        | 5cfb1cf68958 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.14/24 | 2001:172:20:20::e/64  |
|  9 | clab-lab24-leaf3        | c438f734e44d | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.19/24 | 2001:172:20:20::13/64 |
| 10 | clab-lab24-leaf4        | ae4321825a03 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.17/24 | 2001:172:20:20::11/64 |
| 11 | clab-lab24-leaf5        | ee7a520fd844 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.18/24 | 2001:172:20:20::12/64 |
| 12 | clab-lab24-leaf6        | 59c3c515ef35 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.9/24  | 2001:172:20:20::9/64  |
| 13 | clab-lab24-leaf7        | 111f858b19fd | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.22/24 | 2001:172:20:20::16/64 |
| 14 | clab-lab24-leaf8        | 0ecc69891eb4 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.20/24 | 2001:172:20:20::14/64 |
| 15 | clab-lab24-prometheus   | 357821ec726e | prom/prometheus:latest       | linux |       | running | 172.20.20.5/24  | 2001:172:20:20::5/64  |
| 16 | clab-lab24-spine1       | 0f5f6f6dc5fa | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.13/24 | 2001:172:20:20::d/64  |
| 17 | clab-lab24-spine2       | b718503d3b3f | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.15/24 | 2001:172:20:20::f/64  |
| 18 | clab-lab24-spine3       | e02f18d0e3ff | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.11/24 | 2001:172:20:20::b/64  |
| 19 | clab-lab24-spine4       | 3347cba3f277 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.12/24 | 2001:172:20:20::c/64  |
| 20 | clab-lab24-super-spine1 | 4abc7bcaf43c | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.16/24 | 2001:172:20:20::10/64 |
| 21 | clab-lab24-super-spine2 | 5b2f5f153d43 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.21/24 | 2001:172:20:20::15/64 |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
```
Check the [Prometheus Output](../../../user_guide/outputs/prometheus_output.md) and [gNMI Server](../../../user_guide/gnmi_server.md) documentation pages for more configuration options
