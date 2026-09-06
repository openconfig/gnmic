<p align=center><img src=docs/images/gnmic-headline.svg?sanitize=true/></p>

[![github release](https://img.shields.io/github/release/openconfig/gnmic.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/openconfig/gnmic/releases/)
[![Github all releases](https://img.shields.io/github/downloads/openconfig/gnmic/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/openconfig/gnmic/releases/)
[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/openconfig/gnmic)
[![Doc](https://img.shields.io/badge/Docs-gnmic.openconfig.net-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://gnmic.openconfig.net)
[![build](https://img.shields.io/github/actions/workflow/status/openconfig/gnmic/test.yml?branch=main&style=flat-square&labelColor=bec8d2)](https://github.com/openconfig/gnmic/releases/)

---

`gnmic` (_pronoun.: gee·en·em·eye·see_) is a gNMI CLI client that provides full support for Capabilities, Get, Set and Subscribe RPCs with collector capabilities.

Documentation available at [https://gnmic.openconfig.net](https://gnmic.openconfig.net)

## Documentation development

The site uses [Zensical](https://zensical.org/) with its modern theme and the
existing [`mkdocs.yml`](mkdocs.yml) configuration. To start a live-reloading
preview at <http://127.0.0.1:8000>, run:

```sh
make serve-docs
```

On Linux, macOS, or Windows via WSL, you need `make` and either `curl` or `wget`.
The first run downloads a pinned version of [uv](https://docs.astral.sh/uv/),
automatically detects your OS and architecture, and installs Python if needed.
The uv binary, downloaded Python, dependencies, and package cache stay under
the git-ignored `.tools/docs/` directory; shell profiles are left untouched.
Internet access is required for the initial setup. Later runs reuse these files.
Zensical's build cache (`.cache/`) and output (`site/`) are also git-ignored.

```sh
make serve-docs DOCS_ADDR=0.0.0.0:8000  # Listen on all interfaces, e.g. in a dev container
make build-docs                       # Clean, strict production build in site/
make update-docs                      # Update the locked documentation dependencies
```

Commit `uv.lock` after reviewing an upgrade with `make build-docs` and the local
preview. Normal builds use `uv run --locked` so local development and CI use the
same dependency versions. The Python version is selected by `.python-version`;
the uv version is pinned in `scripts/docs.sh`.

The documentation workflow runs `make deploy-docs`, which builds with Zensical
and publishes `site/` to `gh-pages` using `ghp-import` through the same uv wrapper.
It retains `docs/CNAME` and publishes on `docs-*` branch pushes, `v*` tags, and
manual workflow runs. GitHub Pages continues to serve the root of `gh-pages`.
`make deploy-docs` also works locally with repository push access and a configured
Git author; it force-pushes the generated site to `origin/gh-pages`.

### Search tags

Pages use Zensical's native [search tags](https://zensical.org/docs/setup/tags/),
following [containerlab's approach](https://github.com/srl-labs/containerlab/blob/main/mkdocs.yml).
Tag filters connect related guides and examples: search for `Prometheus`, then
select `Output` for configuration reference or `Deployment` for runnable examples.
Search results also highlight matching terms when you open a page.

When adding a page, include YAML front matter with a category and the relevant
topics or integrations, for example:

```yaml
---
tags:
  - Output
  - Prometheus
  - Remote write
---
```

Reuse the spelling and capitalization listed in `extra.tags` in `mkdocs.yml`.
Use `Command` for CLI reference, `Input` or `Output` for integration reference,
`Event processor` for processors, and `Deployment` for deployment examples.
Examples also carry their platform (`Containerlab`, `Docker Compose`, or
`Kubernetes`) and any applicable `Clustering` or `Pipeline` tags. Add integration
tags to both their reference pages and the examples that use them. Keep tags
focused on the page's subject; mentions of a feature do not need their own tag.
If a new topic needs a tag, register it in `extra.tags` and reuse an appropriate
icon identifier from `theme.icon.tag`.

Tags are indexed automatically, including on pages with `hide: [tags]`. No
additional plugin is needed. The unfinished blog is excluded with
`search: {exclude: true}`; remove that front matter when it has useful content.
After editing tags, run `make build-docs` and check the search filters in the
local preview.

## Features

* **Full support for gNMI RPCs**  
  Every gNMI RPC has a [corresponding command](https://gnmic.openconfig.net/basic_usage/) with all of the RPC options configurable by means of the local and global flags.
* **Flexible collector deployment**  
  `gnmic` can be deployed as a gNMI collector that supports multiple output types ([NATS](https://gnmic.openconfig.net/user_guide/outputs/nats_output/), [Kafka](https://gnmic.openconfig.net/user_guide/outputs/kafka_output/), [Prometheus](https://gnmic.openconfig.net/user_guide/outputs/prometheus_output/), [InfluxDB](https://gnmic.openconfig.net/user_guide/outputs/influxdb_output/),...).  
  The collector can be deployed either as a [single instance](https://gnmic.openconfig.net/deployments/deployments_intro/#single-instance), as part of a [cluster](https://gnmic.openconfig.net/user_guide/HA/), or used to form [data pipelines](https://gnmic.openconfig.net/deployments/deployments_intro/#pipelines).
* **Support gRPC tunnel based dialout telemetry**  
  `gnmic` can be deployed as a gNMI collector with an [embedded tunnel server](https://gnmic.openconfig.net/user_guide/tunnel_server/).
* **gNMI data manipulation**  
  `gnmic` collector has [data transformation](https://gnmic.openconfig.net/user_guide/event_processors/intro/) capabilities that can be used to adapt the collected data to your specific use case.
* **Dynamic targets loading**  
  `gnmic` support [target loading at runtime](https://gnmic.openconfig.net/user_guide/targets/target_discovery/discovery_intro/) based on input from external systems.
* **YANG-based path suggestions**  
  Your CLI magically becomes a YANG browser when `gnmic` is executed in [prompt](https://gnmic.openconfig.net/user_guide/prompt_suggestions/) mode. In this mode the flags that take XPATH values will get auto-suggestions based on the provided YANG modules. In other words - voodoo magic :exploding_head:
* **Multi-target operations**  
  Commands can operate on [multiple gNMI targets](https://gnmic.openconfig.net/user_guide/targets/) for bulk configuration/retrieval/subscription.
* **Multiple configuration sources**  
  gnmic supports [flags](https://gnmic.openconfig.net/user_guide/configuration_flags), [environment variables](https://gnmic.openconfig.net/user_guide/configuration_env/) as well as [file based]((https://gnmic.openconfig.net/user_guide/configuration_file/)) configurations.
* **Inspect raw gNMI messages**  
  With the `prototext` output format you can see the actual gNMI messages being sent/received. Its like having a gNMI looking glass!
* **(In)secure gRPC connection**  
  gNMI client supports both TLS and [non-TLS](https://gnmic.openconfig.net/global_flags/#insecure) transports so you can start using it in a lab environment without having to care about the PKI.
* **Dial-out telemetry**  
  The [dial-out telemetry server](https://gnmic.openconfig.net/cmd/listen/) is provided for Nokia SR OS.
* **Pre-built multi-platform binaries**  
  Statically linked [binaries](https://github.com/openconfig/gnmic/releases) made in our release pipeline are available for major operating systems and architectures. Making [installation](https://gnmic.openconfig.net/install/) a breeze!
* **Extensive and friendly documentation**  
  You won't be in need to dive into the source code to understand how `gnmic` works, our [documentation site](https://gnmic.openconfig.net) has you covered.

## Quick start guide

### Installation

```
bash -c "$(curl -sL https://get-gnmic.openconfig.net)"
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

### Prompt mode

The [prompt mode](https://gnmic.openconfig.net/user_guide/prompt_suggestions/) is an interactive mode of the gnmic CLI client for user convenience.

```bash
# clone repository with YANG models (Openconfig example)
git clone https://github.com/openconfig/public
cd public

# Start gnmic in prompt mode and read in all the modules:

gnmic --file release/models \
      --dir third_party \
      --exclude ietf-interfaces \
      prompt
```
