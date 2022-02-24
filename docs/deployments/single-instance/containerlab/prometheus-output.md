The purpose of this deployment is to collect gNMI data and make it available for scraping by a `Prometheus` client.

This deployment example includes a single `gnmic` instance, a [Prometheus Server](https://prometheus.io/), a [Consul agent](https://www.consul.io/docs/agent) used by Prometheus to discover gNMIc's [Prometheus output](../../../user_guide/outputs/prometheus_output.md) and a [Grafana](https://grafana.com/docs/) server.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:3,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/clab_deployments.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fclab_deployments.drawio" async></script>

Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/4.prometheus-output/containerlab/prometheus.clab.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/4.prometheus-output/containerlab/gnmic.yaml)

- [Prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/4.prometheus-output/containerlab/prometheus/prometheus.yaml)

- [Grafana datasource](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/4.prometheus-output/containerlab/grafana/datasources/datasource.yaml)

The deployed SR Linux nodes are discovered using Docker API and are loaded as gNMI targets.
Edit the subscriptions section if needed.

Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/1.single-instance/4.prometheus-output/containerlab
sudo clab deploy -t prometheus.clab.yaml
```

```text
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
| #  |          Name           | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address   |     IPv6 Address     |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
|  1 | clab-lab14-consul-agent | e402b0516753 | consul:latest                | linux |       | running | 172.20.20.4/24  | 2001:172:20:20::4/64 |
|  2 | clab-lab14-gnmic        | 53943cdb8cde | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.3/24  | 2001:172:20:20::3/64 |
|  3 | clab-lab14-grafana      | 1a57efb74f37 | grafana/grafana:latest       | linux |       | running | 172.20.20.2/24  | 2001:172:20:20::2/64 |
|  4 | clab-lab14-leaf1        | 8343848fbd7a | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.9/24  | 2001:172:20:20::9/64 |
|  5 | clab-lab14-leaf2        | 9986ff987048 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.8/24  | 2001:172:20:20::8/64 |
|  6 | clab-lab14-leaf3        | 25a212fcb7a1 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.11/24 | 2001:172:20:20::b/64 |
|  7 | clab-lab14-leaf4        | 025373e9f192 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.10/24 | 2001:172:20:20::a/64 |
|  8 | clab-lab14-prometheus   | ae9b47c49c8d | prom/prometheus:latest       | linux |       | running | 172.20.20.5/24  | 2001:172:20:20::5/64 |
|  9 | clab-lab14-spine1       | fb9abd5b4c5c | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.7/24  | 2001:172:20:20::7/64 |
| 10 | clab-lab14-spine2       | f32906f19d55 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.6/24  | 2001:172:20:20::6/64 |
+----+-------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
```

Check the [Prometheus output](../../../user_guide/outputs/prometheus_output.md) documentation page for more configuration options.
