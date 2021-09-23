There are numerous ways `gnmic` can be deployed, each fulfilling a specific use case. 

Whether it is gNMI telemetry collection and export to a single output, 
or clustered data pipelines with high availability and redundancy, 
the below examples should cover the most common use cases.

In this section you will find multiple deployment examples, using [docker-compose](https://docs.docker.com/compose/) or [containerlab](https://containerlab.srlinux.dev/).
Each deployment comes with:

- a `docker-compose` or `clab` file 
- one or multiple `gnmic` configuration file(s)
- extra configuration files if required by the use case (e.g: prometheus, grafana,...)

The [containerlab](https://containerlab.srlinux.dev/) examples come with a fabric deployed using Nokia's [SR Linux](https://learn.srlinux.dev)

If you don't find an example that fits your needs, feel free to start a discussion on [github](https://github.com/karimra/gnmic/discussions)
### Single Instance

These examples showcase single `gnmic` instance deployments with the most commonly used outputs

- NATS output: [clab](single-instance/containerlab/nats-output.md), [docker-compose](single-instance/docker-compose/nats-output.md) 
- Kafka output: [clab](single-instance/containerlab/kafka-output.md), [docker-compose](single-instance/docker-compose/kafka-output.md)
- InfluxDB output: [clab](single-instance/containerlab/influxdb-output.md), [docker-compose](single-instance/docker-compose/influxdb-output.md)
- Prometheus output: [clab](single-instance/containerlab/prometheus-output.md), [docker-compose](single-instance/docker-compose/prometheus-output.md)
- Multiple outputs: [clab](single-instance/containerlab/multiple-outputs.md), [docker-compose](single-instance/docker-compose/multiple-outputs.md)

### Clusters

`gnmic` can also be deployed in [clustered mode](../user_guide/HA.md) to either load share the targets connections between multiple instances and offer connection resiliency,
and/or replicate the collected data among all the cluster members

- InfluxDB output: [clab](clusters/containerlab/cluster_with_influxdb_output.md), [docker-compose](clusters/docker-compose/cluster_with_influxdb_output.md)
- Prometheus output: [clab](clusters/containerlab/cluster_with_prometheus_output.md), [docker-compose](clusters/docker-compose/cluster_with_prometheus_output.md)
- Prometheus output with data replication: [clab](clusters/containerlab/cluster_with_nats_input_and_prometheus_output.md), [docker-compose](clusters/docker-compose/cluster_with_nats_input_and_prometheus_output.md)

### Pipelines

Building data pipelines using `gnmic` is achieved using the [outputs](../user_guide/outputs/output_intro.md) and [inputs](../user_guide/inputs/input_intro.md) plugins.

You will be able to process the data in a serial fashion, split it for parallel processing or mirror it to create a forked pipeline.

- NATS to Prometheus: [docker-compose](pipelines/docker-compose/nats_prometheus.md)
- NATS to InfluxDB: [docker-compose](pipelines/docker-compose/nats_influxdb.md)
- Clustered pipeline: [docker-compose](pipelines/docker-compose/gnmic_cluster_nats_prometheus.md)
- Forked pipeline: [docker-compose](pipelines/docker-compose/forked_pipeline.md)
