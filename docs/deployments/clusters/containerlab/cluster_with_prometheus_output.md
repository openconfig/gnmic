The purpose of this deployment is to achieve __redundancy__, __high-availability__ via clustering.

This deployment example includes:

- A 3 instances [gNMIc cluster](../../../user_guide/HA.md),
- A [Prometheus](https://prometheus.io/) Server
- A [Grafana](https://grafana.com/docs/) Server
- A [Consul](https://www.consul.io/docs/intro) Server

The leader election and target distribution is done with the help of a [Consul server](https://www.consul.io/docs/introhttps://www.consul.io/docs/intro)

`gnmic` will also register its Prometheus output service in `Consul` so that Prometheus can discover which Prometheus servers are available to be scraped.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/clab_cluster_deployments&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fclab_cluster_deployments" async></script>



Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/containerlab/lab22.clab.yaml)
- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/containerlab/gnmic.yaml)
- [Prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/containerlab/prometheus/prometheus.yaml)
- [Grafana datasource](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/containerlab/grafana/datasources/datasource.yaml)

Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/2.clusters/2.prometheus-output/containerlab
sudo clab deploy -t lab22.clab.yaml
```

```text
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
| #  |          Name           | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address   |     IPv6 Address      |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
|  1 | clab-lab22-consul-agent | 542169159f8b | consul:latest                | linux |       | running | 172.20.20.2/24  | 2001:172:20:20::2/64  |
|  2 | clab-lab22-gnmic1       | c04b2b597e7a | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.4/24  | 2001:172:20:20::4/64  |
|  3 | clab-lab22-gnmic2       | 49604280d82d | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.3/24  | 2001:172:20:20::3/64  |
|  4 | clab-lab22-gnmic3       | 49e910460cad | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.5/24  | 2001:172:20:20::5/64  |
|  5 | clab-lab22-grafana      | c0a37b012d29 | grafana/grafana:latest       | linux |       | running | 172.20.20.7/24  | 2001:172:20:20::7/64  |
|  6 | clab-lab22-leaf1        | c6429b499c11 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.19/24 | 2001:172:20:20::13/64 |
|  7 | clab-lab22-leaf2        | 62f235b39a62 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.17/24 | 2001:172:20:20::11/64 |
|  8 | clab-lab22-leaf3        | 78d3b4e62a6b | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.11/24 | 2001:172:20:20::b/64  |
|  9 | clab-lab22-leaf4        | 8c5d80b4d916 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.13/24 | 2001:172:20:20::d/64  |
| 10 | clab-lab22-leaf5        | 508d4d2389b4 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.16/24 | 2001:172:20:20::10/64 |
| 11 | clab-lab22-leaf6        | 14ce19a8c5da | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.9/24  | 2001:172:20:20::9/64  |
| 12 | clab-lab22-leaf7        | c4f6e586baa3 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.20/24 | 2001:172:20:20::14/64 |
| 13 | clab-lab22-leaf8        | 1e00e6346bf1 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.12/24 | 2001:172:20:20::c/64  |
| 14 | clab-lab22-prometheus   | 5ed38ce63113 | prom/prometheus:latest       | linux |       | running | 172.20.20.6/24  | 2001:172:20:20::6/64  |
| 15 | clab-lab22-spine1       | 38247b0f81e7 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.10/24 | 2001:172:20:20::a/64  |
| 16 | clab-lab22-spine2       | 76bf66748acd | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.21/24 | 2001:172:20:20::15/64 |
| 17 | clab-lab22-spine3       | 5c8776e2fc77 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.15/24 | 2001:172:20:20::f/64  |
| 18 | clab-lab22-spine4       | de67e5b92f36 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.14/24 | 2001:172:20:20::e/64  |
| 19 | clab-lab22-super-spine1 | 00f0aee0265a | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.18/24 | 2001:172:20:20::12/64 |
| 20 | clab-lab22-super-spine2 | 418888eb7325 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.8/24  | 2001:172:20:20::8/64  |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
```
Check the [Prometheus Output](../../../user_guide/outputs/prometheus_output.md) documentation page for more configuration options.
