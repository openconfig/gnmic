<nbsp/>
<p style="text-align:center;"><img src=https://raw.githubusercontent.com/karimra/gnmic/main/docs/images/gnmic-headline.svg?sanitize=true/></p>

[![github release](https://img.shields.io/github/release/karimra/gnmic.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/karimra/gnmic/releases/)
[![Github all releases](https://img.shields.io/github/downloads/karimra/gnmic/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/karimra/gnmic/releases/)

---

`gnmic` <small>_(pronoun.: gee·en·em·eye·see)_</small> is a gNMI CLI client that provides full support for Capabilities, Get, Set and Subscribe RPCs with collector capabilities.

## Features
* **Full support for gNMI RPCs**  
  Every gNMI RPC has a [corresponding command](https://gnmic.kmrd.dev/basic_usage/) with all of the RPC options configurable by means of the local and global flags.
* **Flexible collector deployment**  
  `gnmic` can be deployed as a gNMI collector that supports multiple output types ([NATS](user_guide/outputs/nats_output.md), [Kafka](user_guide/outputs/kafka_output.md), [Prometheus](user_guide/outputs/prometheus_output.md), [InfluxDB](user_guide/outputs/influxdb_output.md),...).  
  The collector can be deployed either as a [single instance](deployments/deployments_intro/#single-instance), as part of a [cluster](user_guide/HA/), or used to form [data pipelines](deployments/deployments_intro/#pipelines).
* **gNMI data manipulation**   
  `gnmic` collector supports [data transformation](user_guide/event_processors/intro/) capabilities that can be used to adapt the collected data to your specific use case.
* **Dynamic targets loading**  
  `gnmic` support [target loading at runtime](user_guide/target_discovery/discovery_intro.md) based on input from external systems.
* **YANG-based path suggestions**  
  Your CLI magically becomes a YANG browser when `gnmic` is executed in [prompt](user_guide/prompt_suggestions.md) mode. In this mode the flags that take XPATH values will get auto-suggestions based on the provided YANG modules. In other words - voodoo magic :exploding_head:
* **Multiple configuration sources**  
  gnmic supports [flags](user_guide/configuration_flags), [environment variables](user_guide/configuration_env/) as well as [file based](https://gnmic.kmrd.dev/user_guide/configuration_file/) configurations.
* **Multi-target operations**  
  Commands can operate on [multiple gNMI targets](https://gnmic.kmrd.dev/user_guide/targets/) for bulk configuration/retrieval/subscription.
* **Multiple subscriptions**  
  With file based configuration it is possible to define and configure [multiple subscriptions](https://gnmic.kmrd.dev/user_guide/subscriptions/) which can be independently associated with gNMI targets.
* **Inspect gNMI messages**  
  With the `textproto` output format and the logging capabilities of `gnmic` you can see the actual gNMI messages being sent/received. Its like having a gNMI looking glass!
* **Configurable TLS enforcement**  
  gNMI client supports both TLS and [non-TLS](https://gnmic.kmrd.dev/global_flags/#insecure) transports so you can start using it in a lab environment without having to care about the PKI.
* **Dial-out telemetry**  
  The [dial-out telemetry server](https://gnmic.kmrd.dev/cmd/listen/) is provided for Nokia SR OS.
* **Pre-built multi-platform binaries**  
  Statically linked [binaries](https://github.com/karimra/gnmic/releases) made in our release pipeline are available for major operating systems and architectures. Making [installation](https://gnmic.kmrd.dev/install/) a breeze!
* **Extensive and friendly documentation**  
  You won't be in need to dive into the source code to understand how `gnimc` works, our [documentation site](https://gnmic.kmrd.dev) has you covered.

## Quick start guide
### Installation
```
bash -c "$(curl -sL https://get-gnmic.kmrd.dev)"
```
### Capabilities request
```
gnmic -a 10.1.0.11:57400 -u admin -p admin --insecure capabilities
```

### Get request
```
gnmic -a 10.1.0.11:57400 -u admin -p admin --insecure \
      get --path /state/system/platform
```

### Set request
```
gnmic -a 10.1.0.11:57400 -u admin -p admin --insecure \
      set --update-path /configure/system/name \
          --update-value gnmic_demo
```

### Subscribe request
```
gnmic -a 10.1.0.11:57400 -u admin -p admin --insecure \
      sub --path "/state/port[port-id=1/1/c1/1]/statistics/in-packets"
```