The purpose of this deployment is to collect gNMI data and write it to a `NATS` server.

Multiple 3rd Party systems (acting as a NATS clients) can then read the data from the `NATS` server for further processing.

This deployment example includes a single `gnmic` instance and a single [NATS output](../../../user_guide/outputs/nats_output.md)


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/clab_deployments.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fclab_deployments.drawio" async></script>

Deployment files:

- [containerlab](https://github.com/karimra/gnmic/tree/main/examples/deployments/1.single-instance/1.nats-output/containerlab/nats.clab.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/tree/main/examples/deployments/1.single-instance/1.nats-output/containerlab/gnmic.yaml)

The deployed SR Linux nodes are discovered using Docker API and are loaded as gNMI targets.
Edit the subscriptions section if needed.

Deploy it with:

```bash
git clone https://github.com/karimra/gnmic.git
cd gnmic/examples/deployments/1.single-instance/1.nats-output/containerlab
sudo clab deploy -t nats.clab.yaml
```

```text
+---+-------------------+--------------+------------------------------+-------+-------+---------+----------------+----------------------+
| # |       Name        | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-------------------+--------------+------------------------------+-------+-------+---------+----------------+----------------------+
| 1 | clab-lab11-gnmic  | 955eaa35b730 | ghcr.io/karimra/gnmic:latest | linux |       | running | 172.20.20.3/24 | 2001:172:20:20::3/64 |
| 2 | clab-lab11-leaf1  | f0f61a79124e | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.4/24 | 2001:172:20:20::4/64 |
| 3 | clab-lab11-leaf2  | de714ee79856 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.9/24 | 2001:172:20:20::9/64 |
| 4 | clab-lab11-leaf3  | c674b7bbb898 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.8/24 | 2001:172:20:20::8/64 |
| 5 | clab-lab11-leaf4  | c37033f30e99 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.7/24 | 2001:172:20:20::7/64 |
| 6 | clab-lab11-nats   | ebbd346d2aee | nats:latest                  | linux |       | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
| 7 | clab-lab11-spine1 | 0fe91271bdfe | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.6/24 | 2001:172:20:20::6/64 |
| 8 | clab-lab11-spine2 | 6b05f4e42cc4 | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.5/24 | 2001:172:20:20::5/64 |
+---+-------------------+--------------+------------------------------+-------+-------+---------+----------------+----------------------+
```

Check the [NATS Output](../../../user_guide/outputs/nats_output.md) documentation page for more configuration options.
