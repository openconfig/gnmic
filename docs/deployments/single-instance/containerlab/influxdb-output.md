The purpose of this deployment is to collect gNMI data and write it to an `InfluxDB` instance.

This deployment example includes a single `gnmic` instance, a single [InfluxDB](https://www.influxdata.com/) server acting as an [InfluxDB output](../../user_guide/outputs/influxdb_output.md) as well as 2 [SR Linux](https://learn.srlinux.dev/) instances

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/single_instance_influxdb.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fsingle_instance_influxdb.drawio" async></script>


Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/master/examples/deployments/containerlab/1.single-instance/3.influxdb-output/influxdb.clab.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/blob/master/examples/deployments/containerlab/1.single-instance/3.influxdb-output/gnmic.yaml)

- [Grafana datasource config](https://github.com/karimra/gnmic/blob/master/examples/deployments/containerlab/1.single-instance/3.influxdb-output/grafana/datasources/datasource.yaml)

Download both files, update the `gnmic` config file with the desired subscriptions and targets.

Deploy it with:

```bash
git clone github.com/karimra/gnmic
cd gnmic/examples/deployments/containerlab/1.single-instance/3.influxdb-output
sudo clab deploy -t influxdb.clab.yaml
```

```text
+---+---------------------+--------------+------------------------------+-------+-------+---------+----------------+----------------------+
| # |        Name         | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+---------------------+--------------+------------------------------+-------+-------+---------+----------------+----------------------+
| 1 | clab-lab13-gnmic1   | c2c8b417c711 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
| 2 | clab-lab13-grafana  | 718406abb604 | grafana/grafana:latest       | linux |       | running | 172.20.20.4/24 | 2001:172:20:20::4/64 |
| 3 | clab-lab13-influxdb | e4048e5514f7 | influxdb:latest              | linux |       | running | 172.20.20.3/24 | 2001:172:20:20::3/64 |
| 4 | clab-lab13-srl1     | aafaa710f0e5 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.5/24 | 2001:172:20:20::5/64 |
| 5 | clab-lab13-srl2     | 370ca43af7af | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.6/24 | 2001:172:20:20::6/64 |
+---+---------------------+--------------+------------------------------+-------+-------+---------+----------------+----------------------+
```

Check the [InfluxDB Output](../../../user_guide/outputs/influxdb_output.md) documentation page for more configuration options