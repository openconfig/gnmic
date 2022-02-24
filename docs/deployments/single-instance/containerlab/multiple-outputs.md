The purpose of this deployment is to collect gNMI data and write it to multiple outputs.

This deployment example includes:

- A single `gnmic` instance
- A [Prometheus](../../../user_guide/outputs/prometheus_output.md) Server
- An [InfluxDB](../../../user_guide/outputs/influxdb_output.md) Server
- A [NATS](../../../user_guide/outputs/nats_output.md) Server
- A [Kafka](../../../user_guide/outputs/kafka_output.md) Server
- A [File](../../../user_guide/outputs/file_output.md) output
- A [Consul Agent](https://www.consul.io/docs/agent)
- A [Grafana Server](https://grafana.com/docs/)


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:4,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/clab_deployments.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fclab_deployments.drawio" async></script>


Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/5.multiple-outputs/containerlab/multiple-outputs.clab.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/5.multiple-outputs/containerlab/gnmic.yaml)

- [Prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/5.multiple-outputs/containerlab/prometheus/prometheus.yaml)

- [Grafana datasource](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/5.multiple-outputs/containerlab/grafana/datasources/datasource.yaml)

Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/1.single-instance/5.multiple-outputs/containerlab
sudo clab deploy -t multiple-outputs.clab.yaml
```

```text
+----+-----------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
| #  |            Name             | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address   |     IPv6 Address     |
+----+-----------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
|  1 | clab-lab15-consul-agent     | 14f864fb1da9 | consul:latest                | linux |       | running | 172.20.20.4/24  | 2001:172:20:20::4/64 |
|  2 | clab-lab15-gnmic            | cfb8bfca7547 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.6/24  | 2001:172:20:20::6/64 |
|  3 | clab-lab15-grafana          | 56c19565e27c | grafana/grafana:latest       | linux |       | running | 172.20.20.2/24  | 2001:172:20:20::2/64 |
|  4 | clab-lab15-influxdb         | f2d0b2186e10 | influxdb:latest              | linux |       | running | 172.20.20.9/24  | 2001:172:20:20::9/64 |
|  5 | clab-lab15-kafka-server     | efe445dbf0f0 | bitnami/kafka:latest         | linux |       | running | 172.20.20.7/24  | 2001:172:20:20::7/64 |
|  6 | clab-lab15-leaf1            | 42d57c79385e | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.10/24 | 2001:172:20:20::a/64 |
|  7 | clab-lab15-leaf2            | e4b041046779 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.11/24 | 2001:172:20:20::b/64 |
|  8 | clab-lab15-leaf3            | ba87204f2678 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.13/24 | 2001:172:20:20::d/64 |
|  9 | clab-lab15-leaf4            | 327461ee913e | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.15/24 | 2001:172:20:20::f/64 |
| 10 | clab-lab15-nats             | 0363dae05edf | nats:latest                  | linux |       | running | 172.20.20.3/24  | 2001:172:20:20::3/64 |
| 11 | clab-lab15-prometheus       | 44611ebe4a03 | prom/prometheus:latest       | linux |       | running | 172.20.20.8/24  | 2001:172:20:20::8/64 |
| 12 | clab-lab15-spine1           | 8b2b430eea87 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.12/24 | 2001:172:20:20::c/64 |
| 13 | clab-lab15-spine2           | 425bea3a243e | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.14/24 | 2001:172:20:20::e/64 |
| 14 | clab-lab15-zookeeper-server | 91b546eb7bf9 | bitnami/zookeeper:latest     | linux |       | running | 172.20.20.5/24  | 2001:172:20:20::5/64 |
+----+-----------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
```

Check the [gnmic outputs](../../../user_guide/outputs/output_intro.md) documentation page for more configuration options.
