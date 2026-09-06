---
title: gNMIc — gNMI client and telemetry collector
template: home.html
hide:
  - navigation
  - toc
tags:
  - Getting started
---

<div class="gnmic-home" markdown>

<section class="gnmic-hero" markdown>


<div class="gnmic-hero-logo">
  <img src="images/gnmic-headline.svg#only-light" alt="gNMIc: Get, Set, Subscribe, Collect" width="390" height="98">
  <img src="images/gnmic-headline-for-dark-bg.svg#only-dark" alt="gNMIc: Get, Set, Subscribe, Collect" width="390" height="98">
</div>

# Streaming telemetry <span>that works.</span> { .gnmic-hero-title }

Query devices, change configuration, and turn gNMI streams into the metrics your tools understand. Start with a single command. Build a telemetry pipeline when you're ready.
{ .gnmic-hero-lead }

<div class="gnmic-hero-actions" markdown>

[Get started :octicons-arrow-right-24:](install.md){ .md-button .md-button--primary }
[Explore deployments](deployments/deployments_intro.md){ .md-button }

</div>

<div class="gnmic-hero-meta" markdown>

[:octicons-mark-github-16: Open source](https://github.com/openconfig/gnmic)
[:octicons-law-16: Apache 2.0](https://github.com/openconfig/gnmic/blob/main/LICENSE)

</div>

<p class="gnmic-platforms">Linux <span>·</span> macOS <span>·</span> Windows via WSL <span>·</span> Containers <span>·</span> K8s operator</p>

</section>

<section class="gnmic-section" aria-labelledby="start-here" markdown>

<!-- <p class="gnmic-eyebrow">Start here</p> -->

## From a single request to a telemetry pipeline { #start-here }

Use the same tool to explore a device, run a collector, or build gNMI into your own application.
{ .gnmic-section-lead }

<div class="grid cards gnmic-paths" markdown>

-   :octicons-terminal-24:{ .gnmic-card-icon } **Explore with the CLI**

    ---

    Discover capabilities, read state, apply configuration, and subscribe to updates from your terminal.

    [Run your first command :octicons-arrow-right-24:](basic_usage.md)

-   :octicons-workflow-24:{ .gnmic-card-icon } **Build a collector**

    ---

    Collect from many devices, process their updates, and send data to the outputs you already use.

    [Configure your collector :octicons-arrow-right-24:](user_guide/configuration_intro.md)

-   :simple-kubernetes:{ .gnmic-card-icon } **Kubernetes operator**

    ---

    Deploy and manage gNMIc collectors with the Kubernetes operator. Define targets, subscriptions, and outputs as Kubernetes resources.

    [Explore the operator :octicons-arrow-right-24:](https://operator.gnmic.dev/)

-   :octicons-code-24:{ .gnmic-card-icon } **Develop with Go**

    ---

    Create targets and compose gNMI requests with a Go API built for your automation workflows.

    [Explore the Go API :octicons-arrow-right-24:](user_guide/golang_package/intro.md)

</div>

</section>

<section class="gnmic-section gnmic-telemetry" aria-labelledby="telemetry-workflow" markdown>

<div class="gnmic-section-heading" markdown>

<!-- <p class="gnmic-eyebrow">Connect your stack</p> -->

## Collect once. Put your data to work. { #telemetry-workflow }

Bring device telemetry into your monitoring and messaging systems. Filter, enrich, and transform updates along the way.
{ .gnmic-section-lead }

</div>

<div class="gnmic-pipeline" markdown="0" role="img" aria-label="Network devices stream gNMI telemetry to gNMIc, which collects, processes, and routes updates to Prometheus, OpenTelemetry, Kafka, InfluxDB, and other outputs.">
<div class="gnmic-pipeline-stage">
    <span class="gnmic-stage-label">Your network</span>
<div class="gnmic-devices" aria-hidden="true">
      <span class="gnmic-device"><i></i><i></i><i></i><b></b></span>
      <span class="gnmic-device"><i></i><i></i><i></i><b></b></span>
      <span class="gnmic-device"><i></i><i></i><i></i><b></b></span>
</div>
    <span class="gnmic-stage-caption">One device or a fleet</span>
</div>
<div class="gnmic-connection" aria-hidden="true"><span>gNMI streams</span><i></i></div>
<div class="gnmic-collector">
    <span class="gnmic-stage-label">Your collector</span>
    <img src="images/gnmic-wordmark.svg#only-light" alt="" width="150" height="37">
    <img src="images/gnmic-wordmark-for-dark-bg.svg#only-dark" alt="" width="150" height="37">
<div class="gnmic-collector-steps"><span>Collect</span><span>Process</span><span>Output</span></div>
</div>
<div class="gnmic-connection" aria-hidden="true"><span>Metrics &amp; events</span><i></i></div>
<div class="gnmic-pipeline-stage">
    <span class="gnmic-stage-label">Your tools</span>
<div class="gnmic-destinations"><span>Prometheus</span><span>OpenTelemetry</span><span>Kafka</span><span>InfluxDB</span></div>
    <span class="gnmic-stage-caption">Multiple outputs. One pipeline.</span>
</div>
</div>

<div class="gnmic-output-links" markdown>

[Prometheus](user_guide/outputs/prometheus_output.md)
[OpenTelemetry](user_guide/outputs/otlp_output.md)
[InfluxDB](user_guide/outputs/influxdb_output.md)
[ClickHouse](user_guide/outputs/clickhouse_output.md)
[Kafka](user_guide/outputs/kafka_output.md)
[NATS](user_guide/outputs/nats_output.md)
[All outputs :octicons-arrow-right-24:](user_guide/outputs/output_intro.md)

</div>

</section>

<section class="gnmic-section" aria-labelledby="features" markdown>

<!-- <p class="gnmic-eyebrow">Built for the way networks work</p> -->

## Explore, automate, and keep collecting { #features }

<div class="gnmic-features" markdown>

<div markdown>

### :octicons-search-24: Find the right path

Browse YANG models with interactive suggestions for paths, commands, and flags.

[Try prompt mode :octicons-arrow-right-24:](user_guide/prompt_suggestions.md)

</div>

<div markdown>

### :octicons-broadcast-24: Discover your targets

Load devices from files, Docker, Consul, or HTTP sources as your environment changes.

[Explore discovery :octicons-arrow-right-24:](user_guide/targets/target_discovery/discovery_intro.md)

</div>

<div markdown>

### :octicons-filter-24: Process your data

Filter events, enrich tags, convert values, and trigger actions before exporting your data.

[Browse processors :octicons-arrow-right-24:](user_guide/event_processors/intro.md)

</div>

<div markdown>

### :octicons-stack-24: Scale your collection

Distribute targets across a cluster, share the work, and recover when a collector becomes unavailable.

[Learn about clustering :octicons-arrow-right-24:](user_guide/HA.md)

</div>

</div>

</section>

<section class="gnmic-section gnmic-quickstart" aria-labelledby="quick-start-guide" markdown>

<div class="gnmic-quickstart-intro" markdown>

<!-- <p class="gnmic-eyebrow">Try it out</p> -->

## Your first connection { #quick-start-guide }

Install the binary and make a request. The examples use a lab device at `10.1.0.11:57400` with TLS disabled; replace the address and credentials with your own.

[Installation options :octicons-arrow-right-24:](install.md)

[Configure TLS :octicons-arrow-right-24:](user_guide/targets/targets_session_sec.md)

</div>

<div class="gnmic-quickstart-examples" markdown>

=== "Install"

    <span id="installation"></span>

    ```bash
    bash -c "$(curl -sL https://get-gnmic.openconfig.net)"
    ```

    One binary. Ready for your terminal, a container, or a collector deployment.

=== "Capabilities"

    <span id="capabilities-request"></span>

    ```bash
    gnmic -a 10.1.0.11:57400 -u admin -p admin \
      --insecure capabilities
    ```

    Discover the models, encodings, and gNMI version supported by your device.

=== "Get"

    <span id="get-request"></span>

    ```bash
    gnmic -a 10.1.0.11:57400 -u admin -p admin \
      --insecure get --path /state/system/platform
    ```

    Read a snapshot of device state with a gNMI path.

=== "Set"

    <span id="set-request"></span>

    ```bash
    gnmic -a 10.1.0.11:57400 -u admin -p admin \
      --insecure set \
      --update-path /configure/system/name \
      --update-value gnmic_demo
    ```

    Apply a configuration update using a path supported by your device.

=== "Subscribe"

    <span id="subscribe-request"></span>

    ```bash
    gnmic -a 10.1.0.11:57400 -u admin -p admin \
      --insecure subscribe \
      --path '/state/port[port-id=1/1/c1/1]/statistics' \
      --sample-interval 10s
    ```

    Stream fresh telemetry as your network changes.

</div>

</section>

<section class="gnmic-next" aria-labelledby="build-next" markdown>

<div markdown>

<p class="gnmic-eyebrow">Make it your own</p>

## Start with a working deployment { #build-next }

Explore Containerlab, Docker Compose, and Kubernetes examples—from a single collector to a clustered pipeline.

</div>

[Find your example :octicons-arrow-right-24:](deployments/deployments_intro.md){ .md-button .md-button--primary }

</section>

<div class="gnmic-community" markdown>

Built in the open, with the network community.

[:octicons-mark-github-16: GitHub](https://github.com/openconfig/gnmic)
[:octicons-issue-opened-16: Report an issue](https://github.com/openconfig/gnmic/issues)
[Read the changelog :octicons-arrow-right-24:](changelog.md)

</div>

</div>
