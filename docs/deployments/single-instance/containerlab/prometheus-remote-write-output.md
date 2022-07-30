The purpose of this deployment is to collect gNMI data and use [Prometheus remote write API](https://grafana.com/blog/2019/03/25/whats-new-in-prometheus-2.8-wal-based-remote-write/) to push it to different monitoring systems like [Prometheus](https://prometheus.io), [Mimir](https://grafana.com/oss/mimir/), [CortexMetrics](https://cortexmetrics.io/), [VictoriaMetrics](https://victoriametrics.com/), [Thanos](https://thanos.io/)...

This deployment example includes a single `gnmic` instance, a [Prometheus Server](https://prometheus.io/), and a [Grafana](https://grafana.com/docs/) server.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:5,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/clab_deployments.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fclab_deployments.drawio" async></script>

Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/6.prometheus-write-output/containerlab/prom_write.clab.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/6.prometheus-write-output/containerlab/gnmic.yaml)

- [Prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/6.prometheus-write-output/containerlab/prometheus/prometheus.yaml)

- [Grafana datasource](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/6.prometheus-write-output/containerlab/grafana/datasources/datasource.yaml)

The deployed SR Linux nodes are discovered using Docker API and are loaded as gNMI targets.
Edit the subscriptions section if needed.

Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/1.single-instance/6.prometheus-write-output/containerlab
sudo clab deploy -t prometheus.clab.yaml
```

```text
+----+-------------------------+--------------+------------------------------+-------+---------+-----------------+--------------+
| #  |          Name           | Container ID |            Image             | Kind  |  State  |  IPv4 Address   | IPv6 Address |
+----+-------------------------+--------------+------------------------------+-------+---------+-----------------+--------------+
|  1 | clab-lab16-consul-agent | 10054b55e722 | consul:latest                | linux | running | 172.19.19.3/24  | N/A          |
|  2 | clab-lab16-gnmic        | 1eeab0771731 | ghcr.io/karimra/gnmic:latest | linux | running | 172.19.19.5/24  | N/A          |
|  3 | clab-lab16-grafana      | fd09146937ef | grafana/grafana:latest       | linux | running | 172.19.19.2/24  | N/A          |
|  4 | clab-lab16-leaf1        | 0c8f5bf7bafb | ghcr.io/nokia/srlinux        | srl   | running | 172.19.19.11/24 | N/A          |
|  5 | clab-lab16-leaf2        | a33868bef0a3 | ghcr.io/nokia/srlinux        | srl   | running | 172.19.19.9/24  | N/A          |
|  6 | clab-lab16-leaf3        | 3fb3b459cd48 | ghcr.io/nokia/srlinux        | srl   | running | 172.19.19.10/24 | N/A          |
|  7 | clab-lab16-leaf4        | bb2cbc064b05 | ghcr.io/nokia/srlinux        | srl   | running | 172.19.19.6/24  | N/A          |
|  8 | clab-lab16-prometheus   | 63b6fb1551de | prom/prometheus:latest       | linux | running | 172.19.19.4/24  | N/A          |
|  9 | clab-lab16-spine1       | 76853ab9c4a8 | ghcr.io/nokia/srlinux        | srl   | running | 172.19.19.8/24  | N/A          |
| 10 | clab-lab16-spine2       | fdf42ca0fec1 | ghcr.io/nokia/srlinux        | srl   | running | 172.19.19.7/24  | N/A          |
+----+-------------------------+--------------+------------------------------+-------+---------+-----------------+--------------+
```

Check the [Prometheus Remote Write output](../../../user_guide/outputs/prometheus_write_output.md) documentation page for more configuration options.
