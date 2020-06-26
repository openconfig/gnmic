<p align=center><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/46e7d1631bd5569e9bf289be9dfa3812/gnmic-headline.svg?sanitize=true/></p>

[![github release](https://img.shields.io/github/release/karimra/gnmiclient.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/karimra/gnmiclient/releases/)
[![Github all releases](https://img.shields.io/github/downloads/karimra/gnmiclient/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/karimra/gnmiclient/releases/)
[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/karimra/gnmiclient)
[![Doc](https://img.shields.io/badge/Docs-gnmiclient.kmrd.dev-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://gnmiclient.kmrd.dev)
[![build](https://img.shields.io/github/workflow/status/karimra/gnmiclient/Test/master?style=flat-square&labelColor=bec8d2)](https://github.com/karimra/gnmiclient/releases/)

---

gNMI CLI client that provides full support for Capabilities, Get, Set and Subscribe RPCs.

Documentation available at [https://gnmiclient.kmrd.dev](https://gnmiclient.kmrd.dev)

## Features
* **Full support for gNMI RPCs**  
  Every gNMI RPC has a [corresponding command](https://gnmiclient.kmrd.dev/basic_usage/) with all of the RPC options configurable by means of the local and global flags.
* **Multi-target operations**  
  Commands can operate on multiple gNMI targets for bulk configuration/retrieval.
* **File based configuration**  
  gNMI Client understands configurations provided in a file. The configuration options are consistent with the CLI flags.
* **Inspect gNMI messages**  
  With the `textproto` output you can see the actual gNMI messages being sent/received. Its like having a gNMI looking glass!
* **(In)secure gRPC connection**  
  gNMI client supports both TLS and non-TLS transports so you can start using it in a lab environment without having to care about the PKI.
* **Dial-out telemetry**  
  The dial-out telemetry server is provided for Nokia SR OS.
* **Pre-built multi-platform binaries**  
  gNMI Client is available for major operating systems and the [installation](https://gnmiclient.kmrd.dev/install/) is a breeze.
* **Extensive and friendly documentation**  
  You won't be in need to dive into the source code to understand how to use the gNMI CLI client, our [documentation site](https://gnmiclient.kmrd.dev) has you covered.

## Usage
```
$ gnmic --help
run gnmi rpcs from the terminal

Usage:
  gnmic [command]

Available Commands:
  capabilities query targets gnmi capabilities
  get          run gnmi get on targets
  help         Help about any command
  listen       listens for telemetry dialout updates from the node
  path         generate gnmi or xpath style from yang file
  set          run gnmi set on targets
  subscribe    subscribe to gnmi updates on targets
  version      show gnmic version
```
