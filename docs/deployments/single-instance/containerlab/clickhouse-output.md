The purpose of this deployment is to collect gNMI data and write it to a [ClickHouse](https://clickhouse.com/) instance.

This deployment example includes a single `gnmic` instance, a ClickHouse server acting as a [ClickHouse output](../../../user_guide/outputs/clickhouse_output.md), and a [Grafana](https://grafana.com/docs/) server for visualization.

Deployment files:

- [containerlab](https://github.com/openconfig/gnmic/blob/main/examples/deployments/1.single-instance/12.clickhouse-output/containerlab/clickhouse.clab.yaml)

- [gNMIc config](https://github.com/openconfig/gnmic/blob/main/examples/deployments/1.single-instance/12.clickhouse-output/containerlab/gnmic.yaml)

- [Grafana datasource](https://github.com/openconfig/gnmic/blob/main/examples/deployments/1.single-instance/12.clickhouse-output/containerlab/grafana/datasources/datasource.yaml)

The deployed SR Linux nodes are discovered using the Docker API and are loaded as gNMI targets.
Edit the subscriptions section if needed.

Deploy it with:

```bash
git clone https://github.com/openconfig/gnmic.git
cd gnmic/examples/deployments/1.single-instance/12.clickhouse-output/containerlab
sudo clab deploy -t clickhouse.clab.yaml
```

```text
+---+------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
| # |         Name           | Container ID |            Image             | Kind  | Group |  State  |  IPv4 Address   |     IPv6 Address     |
+---+------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
| 1 | clab-lab20-clickhouse  | ...          | clickhouse/clickhouse-server | linux |       | running | 172.20.20.x/24  | ...                  |
| 2 | clab-lab20-gnmic       | ...          | ghcr.io/openconfig/gnmic     | linux |       | running | 172.20.20.x/24  | ...                  |
| 3 | clab-lab20-grafana     | ...          | grafana/grafana:latest       | linux |       | running | 172.20.20.x/24  | ...                  |
| 4 | clab-lab20-leaf1       | ...          | ghcr.io/nokia/srlinux        | srl   |       | running | 172.20.20.x/24  | ...                  |
| ... | SR Linux fabric nodes | ...          | ghcr.io/nokia/srlinux        | srl   |       | running | ...             | ...                  |
+---+------------------------+--------------+------------------------------+-------+-------+---------+-----------------+----------------------+
```

Check the [ClickHouse Output](../../../user_guide/outputs/clickhouse_output.md) documentation page for table schema, materialized views, and configuration options.
