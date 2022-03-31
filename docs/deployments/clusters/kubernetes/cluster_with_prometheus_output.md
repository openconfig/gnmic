The purpose of this deployment is to achieve __redundancy__, __high-availability__ using Kubernetes and `gnmic`'s internal clustering mechanism.

This deployment example includes:

- A 3 instances [`gnmic` cluster](../../../user_guide/HA.md),
- A single [Prometheus output](../../../user_guide/outputs/prometheus_output.md)

The leader election and target distribution is done with the help of a [Consul server](https://www.consul.io/docs/introhttps://www.consul.io/docs/intro)

`gnmic` can be discovered by `Prometheus` using Kubernetes service discovery. Kubernetes uses a [headless service](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services) with a StatefulSet to disable the internal load balancing across multiple pods of the same StatefulSet and allow `Prometheus` to discover all instances of `gnmic`.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/cluster_prometheus_kubernetes&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Fcluster_prometheus_kubernetes" async></script>

<ins>[Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator#quickstart) must be installed prior to `gnmic` deployment.</ins> (Can also be installed via [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) helm chart or [kube-prometheus](https://github.com/prometheus-operator/kube-prometheus))

Deployment files:

- [gnmic](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/kubernetes/gnmic-app)
- [consul](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/kubernetes/consul)
- [prometheus servicemonitor](https://github.com/karimra/gnmic/blob/main/examples/deployments/2.clusters/2.prometheus-output/kubernetes/prometheus/servicemonitor.yaml)

Download the files, update the `gnmic` ConfigMap with the desired subscriptions and targets and make sure that `prometheus servicemonitor` is in a namespace or has a label that `Prometheus operator` is watching.

Deploy it with:

```bash
kubectl create ns gnmic
kubectl apply -n gnmic -f kubernetes/consul
kubectl apply -n gnmic -f kubernetes/gnmic-app
# Before deploying the Prometheus ServiceMonitor
# Install Prometheus operator or kube-prometheus or kube-prometheus-stack helm chart
# Otherwise the command will fail
kubectl apply -f kubernetes/prometheus
```

Check the [Prometheus Output](../../../user_guide/outputs/prometheus_output.md) documentation page for more configuration options.
