The purpose of this deployment is to collect gNMI data and write it to a `Kafka` broker.

Multiple 3rd Party systems (acting as a Kafka consumers) can then read the data from the `Kafka` broker for further processing.

This deployment example includes a single `gnmic` instance and a single [Kafka output](../../../user_guide/outputs/kafka_output.md)

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/clab_deployments.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fclab_deployments.drawio" async></script>

Deployment files:

- [containerlab](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/2.kafka-output/containerlab/kafka.clab.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/2.kafka-output/containerlab/gnmic.yaml)

The deployed SR Linux nodes are discovered using Docker API and are loaded as gNMI targets.
Edit the subscriptions section if needed.

Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/1.single-instance/2.kafka-output/containerlab
sudo clab deploy -t kafka.clab.yaml
```

```text
+---+-----------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
| # |            Name             | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address   |     IPv6 Address     |
+---+-----------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
| 1 | clab-lab12-gnmic            | e79d31f92a7a | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.2/24  | 2001:172:20:20::2/64 |
| 2 | clab-lab12-kafka-server     | 004a338cdb3d | bitnami/kafka:latest         | linux |       | running | 172.20.20.4/24  | 2001:172:20:20::4/64 |
| 3 | clab-lab12-leaf1            | b9269bac3adf | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.7/24  | 2001:172:20:20::7/64 |
| 4 | clab-lab12-leaf2            | baaeea0ad1a6 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.8/24  | 2001:172:20:20::8/64 |
| 5 | clab-lab12-leaf3            | 08127014b3cd | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.5/24  | 2001:172:20:20::5/64 |
| 6 | clab-lab12-leaf4            | da037997c5ff | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.10/24 | 2001:172:20:20::a/64 |
| 7 | clab-lab12-spine1           | c3bcfe40fcc7 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.9/24  | 2001:172:20:20::9/64 |
| 8 | clab-lab12-spine2           | 842b259d01b0 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.6/24  | 2001:172:20:20::6/64 |
| 9 | clab-lab12-zookeeper-server | 5c89e48fdff1 | bitnami/zookeeper:latest     | linux |       | running | 172.20.20.3/24  | 2001:172:20:20::3/64 |
+---+-----------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
```

Check the [Kafka Output](../../../user_guide/outputs/kafka_output.md) documentation page for more configuration options.
