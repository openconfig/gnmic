<p align=center><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/9fa21b0630653b9a938b1b85bb1439cb/gnmi-headline-1.svg?sanitize=true/></p>

[![github release](https://img.shields.io/github/release/karimra/gnmiclient.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/karimra/gnmiclient/releases/)
[![Github all releases](https://img.shields.io/github/downloads/karimra/gnmiclient/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/karimra/gnmiclient/releases/)
[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/karimra/gnmiclient)
[![Doc](https://img.shields.io/badge/Docs-gnmiclient.kmrd.dev-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://gnmiclient.kmrd.dev)
[![build](https://img.shields.io/github/workflow/status/karimra/gnmiclient/Test/master?style=flat-square&labelColor=bec8d2)](https://github.com/karimra/gnmiclient/releases/)

---

gNMI CLI client that provides full support for Capabilities, Get, Set and Subscribe RPCs.

Documentation available at [https://gnmiclient.kmrd.dev](https://gnmiclient.kmrd.dev)

## Usage
```
$ gnmiClient --help
run gnmi rpcs from the terminal

Usage:
  gnmiClient [command]

Available Commands:
  capabilities query targets gnmi capabilities
  get          run gnmi get on targets
  help         Help about any command
  listen       listens for telemetry dialout updates from the node
  path         generate gnmi or xpath style from yang file
  set          run gnmi set on targets
  subscribe    subscribe to gnmi updates on targets
  version      show gnmiClient version
```
