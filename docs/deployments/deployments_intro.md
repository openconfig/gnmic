There are numerous ways `gnmic` can be deployed, each fulfilling a specific use case. 

Whether it is gNMI telemetry collection and export to a single output, 
or clustered data pipelines with high availability and redundancy, 
the below examples should cover the most common use cases.

Each example comes with a set of deployment files:

- one or multiple `gnmic` configuration file(s)
- a `docker-compose` file 
- extra configuration files if the used output requires it (e.g: prometheus)

If you do not not find an example that fits your need, feel free to start a discussion on [github](https://github.com/karimra/gnmic/discussions)
### Single Instance
These examples showcase single `gnmic` instance deployments with the most commonly used outputs

- [NATS output](single-instance/nats-output.md) 
- [Kafka output](single-instance/kafka-output.md)
- [InfluxDB output](single-instance/influxdb-output.md)
- [Prometheus output](single-instance/prometheus-output.md)
- [Multiple outputs](single-instance/multiple-outputs.md)


### Clusters
`gnmic` can also be deployed in [clustered mode](../user_guide/HA.md) to either load share the targets connections between multiple instances and offer connection resiliency,
and/or replicate the collected data among all the cluster members

- [InfluxDB output](clusters/cluster_with_influxdb_output.md)
- [Prometheus output](clusters/cluster_with_prometheus_output.md)
- [Prometheus output with data replication](clusters/cluster_with_nats_input_and_prometheus_output.md)


### Pipelines

Building data pipelines using `gnmic` is achieved using the [outputs](../user_guide/outputs/output_intro.md) and [inputs](../user_guide/inputs/intro.md) plugins.

You will be able to process the data in a serial fashion, split it for parallel processing or mirror it to create a forked pipeline.

- [NATS to Prometheus](pipelines/nats_prometheus.md)
- [NATS to InfluxDB](pipelines/nats_influxdb.md)
- [Clustered pipeline](pipelines/gnmic_cluster_nats_prometheus.md)
- [Forked pipeline](pipelines/forked_pipeline.md)