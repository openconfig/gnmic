<p align=center><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/46e7d1631bd5569e9bf289be9dfa3812/gnmic-headline.svg?sanitize=true/></p>

[![github release](https://img.shields.io/github/release/karimra/gnmic.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/karimra/gnmic/releases/)
[![Github all releases](https://img.shields.io/github/downloads/karimra/gnmic/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/karimra/gnmic/releases/)
[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/karimra/gnmic)
[![Doc](https://img.shields.io/badge/Docs-gnmic.kmrd.dev-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://gnmic.kmrd.dev)
[![build](https://img.shields.io/github/workflow/status/karimra/gnmic/Test/master?style=flat-square&labelColor=bec8d2)](https://github.com/karimra/gnmic/releases/)

---

`gnmic` (_pronoun.: gee·en·em·eye·see_) is a gNMI CLI client that provides full support for Capabilities, Get, Set and Subscribe RPCs with collector capabilities.

Documentation available at [https://gnmic.kmrd.dev](https://gnmic.kmrd.dev)

## Features
* **Full support for gNMI RPCs**  
  Every gNMI RPC has a [corresponding command](https://gnmic.kmrd.dev/basic_usage/) with all of the RPC options configurable by means of the local and global flags.
* **Multi-target operations**  
  Commands can operate on [multiple gNMI targets](https://gnmic.kmrd.dev/advanced/multi_target/) for bulk configuration/retrieval/subscription.
* **File based configuration**  
  gnmic supports [configurations provided in a file](https://gnmic.kmrd.dev/advanced/file_cfg/). The configuration options are consistent with the CLI flags.
* **Inspect raw gNMI messages**  
  With the `prototext` output format you can see the actual gNMI messages being sent/received. Its like having a gNMI looking glass!
* **(In)secure gRPC connection**  
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
sudo curl -sL https://github.com/karimra/gnmic/raw/master/install.sh | sudo bash
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

The prompt mode is an interactive mode of the gnmic CLI client for user convenience.
The gnmic subcommands such as capabilities, get, set and subscribe of the gNMI protocol can be used in the prompt mode with tab completion for their flags and the xpath of the YANG data models that you defined.

![gnmic prompt-mode](docs/images/gnmic.prompt-mode.demo.gif)

> This prompt mode is still being developed. The following work items will be implemented.

### code changes for the prompt mode

- Updated path command to generate a schema fil ($HOME/.gnmic.schema) for tab completion of the prompt mode.
- Separated flags initialization func to reset the used subcommand flags in the prompt mode.
- Added prompt-mode subcommand to root command.

### Jobs to do

- add --dir, --file and --exclude flags to the prompt mode for single step schema loading.
- subcommand execution must return success or failure information to the terminal. (e.g. `get` doesn't any info upon `get` operation failure.)
- `Subscribe` subcommand must be run in background on the prompt mode.
- Add command history to the prompt mode