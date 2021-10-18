

### Description

Most `gNMI` targets use YANG as a modeling language for their datastores.
It order to access and manipulate the stored data (`Get`, `Set`, `Subscribe`), a tool should be aware of the underlying YANG model, be able to generate paths pointing to the desired `gNMI` objects as well as building configuration payloads matching data instances on the targets.

The `generate` command takes the target's YANG models as input and generates:

- Paths in `xpath` or `gNMI` formats.
- Configuration payloads that can be used as [update](../cmd/set.md#3-update-with-a-value-from-json-or-yaml-file) or [replace](../cmd/set.md#3-replace-with-a-value-from-json-or-yaml-file) input files for the Set command.
- A Set request file that can be used as a [template](../cmd/set.md#template-based-set-request) with the Set command.

Aliases: `gen`

### Usage

`gnmic [global-flags] generate [local-flags]`

or

`gnmic [global-flags] generate [local-flags] sub-command [sub-command-flags]`

### Persistent Flags

#### output

The `--output` flag specifies the file to which the generated output will be written, defaults to `stdout`

#### json

When used with `generate` command, the `--json` flag, if present changes the output format from YAML to JSON.

When used with `generate path` command, it outputs the path, the leaf **type**, its **description**, its **default value** and if it is a **state leaf** or not in an array of JSON objects.

### Local Flags

#### path

The `--path` flag specifies the path whose payload (JSON/YAML) will be generated.

Defaults to `/`

#### config-only

The `--config-only` flag, if present instruct `gnmic` to generate JSON/YAML payloads from YANG nodes not marked as `config false`.

#### camel-case

The `--camel-case` flag, if present allows to convert all the keys in the generated JSON/YAML paylod to `CamelCase`

#### snake-case

The `--snake-case` flag, if present allows to convert all the keys in the generated JSON/YAML paylod to `snake_case`

### Sub Commands

#### Path

The path sub command is an alias for the [`gnmic path`](../cmd/path.md) command.

#### Set-request

The [set-request](../cmd/generate/generate_set_request.md) sub command generates a Set request file given a list of update and/or replace paths.

### Examples

#### Openconfig

YANG repo: [openconfig/public](https://github.com/openconfig/public)

Clone the OpenConfig repository:

```bash
git clone https://github.com/openconfig/public
cd public
```

```bash
gnmic --encoding json_ietf \
          generate  \
          --file release/models \
          --dir third_party \
          --exclude ietf-interfaces \
          --path /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address
```

```yaml
- config:
    ip: ""
    prefix-length: ""
  ip: ""
  vrrp:
    vrrp-group:
    - config:
        accept-mode: "false"
        advertisement-interval: "100"
        preempt: "true"
        preempt-delay: "0"
        priority: "100"
        virtual-address: ""
        virtual-router-id: ""
      interface-tracking:
        config:
          priority-decrement: "0"
          track-interface: ""
      virtual-router-id: ""
```
