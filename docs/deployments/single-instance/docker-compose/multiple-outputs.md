The purpose of this deployment is to collect gNMI data and write it to multiple outputs.

This deployment example includes:

- A single `gnmic` instance
- A [Prometheus output](../../../user_guide/outputs/prometheus_output.md)
- An [InfluxDB output](../../../user_guide/outputs/influxdb_output.md)
- A [NATS output](../../../user_guide/outputs/nats_output.md)
- A [Kafka output](../../../user_guide/outputs/kafka_output.md)
- A [File output](../../../user_guide/outputs/file_output.md)


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/single_instance_multiple_outputs.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fsingle_instance_multiple_outputs.drawio" async></script>

Deployment files:

- [docker compose](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/5.multiple-outputs/docker-compose/docker-compose.yaml)

- [gnmic config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/5.multiple-outputs/docker-compose/gnmic1.yaml)

Download both files, update the `gnmic` config file with the desired subscriptions and targets.

Deploy it with:

```bash
sudo docker-compose up -d
```

Check the [gnmic outputs](../../../user_guide/outputs/output_intro.md) documentation page for more configuration options
