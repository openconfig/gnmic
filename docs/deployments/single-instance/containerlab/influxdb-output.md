The purpose of this deployment is to collect gNMI data and write it to an `InfluxDB` instance.

This deployment example includes a single `gnmic` instance, a single [InfluxDB](https://www.influxdata.com/) server acting as an [InfluxDB output](../../../user_guide/outputs/influxdb_output.md) and a [Grafana](https://grafana.com/docs/) server
<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/clab_deployments.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fclab_deployments.drawio" async></script>


Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/3.influxdb-output/containerlab/influxdb.clab.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/3.influxdb-output/containerlab/gnmic.yaml)

- [Grafana datasource](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/3.influxdb-output/containerlab/grafana/datasources/datasource.yaml)

The deployed SR Linux nodes are discovered using Docker API and are loaded as gNMI targets.
Edit the subscriptions section if needed.


Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/1.single-instance/3.influxdb-output/containerlab
sudo clab deploy -t influxdb.clab.yaml
```

```text
+---+---------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
| # |        Name         | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address   |     IPv6 Address     |
+---+---------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
| 1 | clab-lab13-gnmic    | 1ee4c75ff443 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.3/24  | 2001:172:20:20::3/64 |
| 2 | clab-lab13-grafana  | a932207780bb | grafana/grafana:latest       | linux |       | running | 172.20.20.2/24  | 2001:172:20:20::2/64 |
| 3 | clab-lab13-influxdb | 0768ba6ca10b | influxdb:latest              | linux |       | running | 172.20.20.4/24  | 2001:172:20:20::4/64 |
| 4 | clab-lab13-leaf1    | e0e2045fca7f | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.7/24  | 2001:172:20:20::7/64 |
| 5 | clab-lab13-leaf2    | 75b8978e734c | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.6/24  | 2001:172:20:20::6/64 |
| 6 | clab-lab13-leaf3    | 7b03eed78f5d | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.5/24  | 2001:172:20:20::5/64 |
| 7 | clab-lab13-leaf4    | 19007ce81e04 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.8/24  | 2001:172:20:20::8/64 |
| 8 | clab-lab13-spine1   | c044fc51196d | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.10/24 | 2001:172:20:20::a/64 |
| 9 | clab-lab13-spine2   | bcfa52ad2772 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.9/24  | 2001:172:20:20::9/64 |
+---+---------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
```

Check the [InfluxDB Output](../../../user_guide/outputs/influxdb_output.md) documentation page for more configuration options.
