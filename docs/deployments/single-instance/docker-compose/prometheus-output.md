The purpose of this deployment is to collect gNMI data and make it available for scraping by a `Prometheus` client.

This deployment example includes a single `gnmic` instance and a single [Prometheus output](../../../user_guide/outputs/prometheus_output.md)

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/single_instance_prometheus.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fsingle_instance_prometheus.drawio" async></script>

Deployment files:

- [Docker Compose](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/4.prometheus-output/docker-compose/docker-compose.yaml)

- [gNMIc config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/4.prometheus-output/docker-compose/gnmic1.yaml)

- [Prometheus config](https://github.com/karimra/gnmic/blob/main/examples/deployments/1.single-instance/4.prometheus-output/docker-compose/prometheus/prometheus.yaml)

Download both files, update the `gnmic` config file with the desired subscriptions and targets.

Deploy it with:

```bash
sudo docker-compose up -d
```

Check the [Prometheus output](../../../user_guide/outputs/prometheus_output.md) documentation page for more configuration options
