The purpose of this deployment is to achieve __redundancy__, __high-availability__ via clustering.

This deployment example includes:

- A 3 instances [gNMIc cluster](../../../user_guide/HA.md),
- A [InfluxDB](https://www.influxdata.com/) Server 
- A [Grafana](https://grafana.com/docs/) Server
- A [Consul](https://www.consul.io/docs/intro) Server

The leader election and target distribution is done with the help of a [Consul server](https://www.consul.io/docs/intro)

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/clab_cluster_deployments&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fclab_cluster_deployments" async></script>


Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/1.influxdb-output/containerlab/lab21.clab.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/1.influxdb-output/containerlab/gnmic.yaml)

- [Grafana datasource](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/1.influxdb-output/containerlab/grafana/datasources/datasource.yaml)

Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/2.clusters/1.influxdb-output/containerlab
sudo clab deploy -t lab21.clab.yaml
```

```text
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
| #  |          Name           | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address   |     IPv6 Address      |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
|  1 | clab-lab21-consul-agent | a6f6eb70965f | consul:latest                | linux |       | running | 172.20.20.7/24  | 2001:172:20:20::7/64  |
|  2 | clab-lab21-gnmic1       | 9758b0761431 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.5/24  | 2001:172:20:20::5/64  |
|  3 | clab-lab21-gnmic2       | 6d6ae91c64bf | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.2/24  | 2001:172:20:20::2/64  |
|  4 | clab-lab21-gnmic3       | 5df100a9fa73 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.4/24  | 2001:172:20:20::4/64  |
|  5 | clab-lab21-grafana      | fe51bda1830c | grafana/grafana:latest       | linux |       | running | 172.20.20.3/24  | 2001:172:20:20::3/64  |
|  6 | clab-lab21-influxdb     | 20712484d835 | influxdb:latest              | linux |       | running | 172.20.20.6/24  | 2001:172:20:20::6/64  |
|  7 | clab-lab21-leaf1        | ce084f636942 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.14/24 | 2001:172:20:20::e/64  |
|  8 | clab-lab21-leaf2        | 5cbaba4bc9ff | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.11/24 | 2001:172:20:20::b/64  |
|  9 | clab-lab21-leaf3        | a5e92ca08c7e | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.8/24  | 2001:172:20:20::8/64  |
| 10 | clab-lab21-leaf4        | 1ccfe0082b15 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.12/24 | 2001:172:20:20::c/64  |
| 11 | clab-lab21-leaf5        | 7fd4144277a0 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.9/24  | 2001:172:20:20::9/64  |
| 12 | clab-lab21-leaf6        | cb4df0d609db | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.13/24 | 2001:172:20:20::d/64  |
| 13 | clab-lab21-leaf7        | 8f09b622365f | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.19/24 | 2001:172:20:20::13/64 |
| 14 | clab-lab21-leaf8        | 0ab91010b4a7 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.18/24 | 2001:172:20:20::12/64 |
| 15 | clab-lab21-spine1       | 86d00f11b944 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.15/24 | 2001:172:20:20::f/64  |
| 16 | clab-lab21-spine2       | 90cf49595ad2 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.20/24 | 2001:172:20:20::14/64 |
| 17 | clab-lab21-spine3       | 1c694820eb88 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.16/24 | 2001:172:20:20::10/64 |
| 18 | clab-lab21-spine4       | 1e3eac3de55f | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.10/24 | 2001:172:20:20::a/64  |
| 19 | clab-lab21-super-spine1 | aafc478de31d | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.21/24 | 2001:172:20:20::15/64 |
| 20 | clab-lab21-super-spine2 | bb27b743c97f | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.17/24 | 2001:172:20:20::11/64 |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+-----------------------+
```

Check the [InfluxDB Output](../../../user_guide/outputs/influxdb_output.md) documentation page for more configuration options.
