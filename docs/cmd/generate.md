

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

#### file
A path to a YANG file or a directory with YANG files which `gnmic` will use to generate paths or JSON/YAML objects.

Multiple `--file` flags can be supplied.

#### dir

A path to a directory which `gnmic` would recursively traverse in search for the additional YANG files which may be required by YANG files specified with `--file` to build the YANG tree.

Can also point to a single YANG file instead of a directory.

Multiple `--dir` flags can be supplied.

#### exclude

The `--exclude` flag specifies the YANG module __names__ to be excluded from the path generation when YANG modules names clash.

Can also be a regular expression.

Multiple `--exclude` flags can be supplied.

#### output

The `--output` flag specifies the file to which the generated output will be written, defaults to `stdout`

#### json

The `--json` flag, if present changes the output format from YAML to JSON.

### Local Flags

#### path

The `--path` flag specifies the path whose payload (JSON/YAML) will be generated.

Defaults to `/`

#### config-only

The `--config-only` flag, if present instruct `gnmic` to generate JSON/YAML payloads from YANG nodes not marked as `config false`.

### Sub Commands

#### Path

The path sub command is an alias for the [`gnmic path`](../cmd/path.md) command.

#### Set-request

The [set-request](../cmd/generate/generate_set_request.md) sub command generates a Set request file given a list of update and/or replace paths.

### Examples
