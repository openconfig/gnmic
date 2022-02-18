## Description

The `set` command represents the [gNMI Set RPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L62).

It is used to send a [Set Request](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L339) to the specified target(s) and expects one [Set Response](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L356) per target.

Set RPC allows the client to [modify the state](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#34-modifying-state) of data on the target. The data specified referenced by a path can be [updated, replaced or deleted](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#343-transactions).

!!! note
    It is possible to combine `update`, `replace` and `delete` in a single `gnmic set` command.

## Usage

`gnmic [global-flags] set [local-flags]`

The Set Request can be any of (or a combination of) update, replace or/and delete operations.

## Flags

### prefix

The `--prefix` flag sets a common [prefix](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#241-path-prefixes) to all paths specified using the local `--path` flag. Defaults to `""`.

If a user needs to provide [origin](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#222-paths) information to the Path message, the following pattern should be used for the path string: `"origin:path"`:

!!! note
    The path after the origin value has to start with a `/`

```bash
gnmic set --update "openconfig-interfaces:/interfaces/interface:::<type>:::<value>"
```

### target

With the optional `[--target]` flag it is possible to supply the [path target](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#2221-path-target) information in the prefix field of the SetRequest message.

### dry-run

The `--dry-run` flag allow to run a Set request without sending it to the targets.
This is useful while developing templated Set requests.

## Update Request

There are several ways to perform an update operation with gNMI Set RPC:

#### 1. in-line update, implicit type
Using both `--update-path` and `--update-value` flags, a user can update a value for a given path.

```bash
gnmic set --update-path /configure/system/name --update-value router1

gnmic set --update-path /configure/router[router-name=Base]/interface[interface-name=system]/admin-state \
          --update-value enable
```

The above 2 updates can be combined in the same cli command:

```bash
gnmic set --update-path /configure/system/name \
          --update-value router1 \
          --update-path /configure/router[router-name=Base]/interface[interface-name=system]/admin-state \
          --update-value enable
```

#### 2. in-line update, explicit type

Using the update flag `--update`, one can specify the path, value type and value in a single parameter using a delimiter `--delimiter`. Delimiter string defaults to `":::"`.

Supported types: json, json_ietf, string, int, uint, bool, decimal, float, bytes, ascii.

```bash
# path:::value-type:::value
gnmic set --update /configure/system/name:::json:::router1

gnmic set --update /configure/router[router-name=Base]/interface[interface-name=system]/admin-state:::json:::enable

gnmic set --update /configure/router[router-name=Base]/interface[interface-name=system]:::json:::'{"admin-state":"enable"}'
```

#### 3. update with a value from JSON or YAML file

It is also possible to specify the values from a local JSON or YAML file using `--update-file` flag for the value and `--update-path` for the path.

In which case the value encoding will be determined by the global flag `[ -e | --encoding ]`, both `JSON` and `JSON_IETF` are supported

The file's format is identified by its extension, json: `.json` and yaml `.yaml` or `.yml`.

=== "interface.json"
    ```bash
    {
        "admin-state": "enable",
        "ipv4": {
            "primary": {
                "address": "1.1.1.1",
                "prefix-length": 32
            }
        }
    }
    ```
    ``` bash
    gnmic set --update-path /configure/router[router-name=Base]/interface[interface-name=system] \
              --update-file interface.json
    ```

=== "interface.yml"

    ```bash
    "admin-state": enable
    "ipv4":
    "primary":
        "address": 1.1.1.1
        "prefix-length": 32
    ```
    ``` bash
    gnmic set --update-path /configure/router[router-name=Base]/interface[interface-name=system] \
              --update-file interface.yml
    ```

## Replace Request

There are 3 main ways to specify a replace operation:

#### 1. in-line replace, implicit type

Using both `--replace-path` and `--replace-value` flags, a user can replace a value for a given path. The type of the value is implicitly set to `JSON`:

```bash
gnmic set --replace-path /configure/system/name --replace-value router1
```

```bash
gnmic set --replace-path /configure/router[router-name=Base]/interface[interface-name=system]/admin-state \
          --replace-value enable
```

The above 2 commands can be packed in the same cli command:

```bash
gnmic set --replace-path /configure/system/name \
          --replace-value router1 \
          --replace-path /configure/router[router-name=Base]/interface[interface-name=system]/admin-state \
          --replace-value enable
```

#### 2. in-line replace, explicit type

Using the replace flag `--replace`, you can specify the path, value type and value in a single parameter using a delimiter `--delimiter`. Delimiter string defaults to `":::"`.

Supported types: json, json_ietf, string, int, uint, bool, decimal, float, bytes, ascii.

```bash
gnmic set --replace /configure/system/name:::json:::router1
```

```bash
gnmic set --replace /configure/router[router-name=Base]/interface[interface-name=system]/admin-state:::json:::enable
```

#### 3. replace with a value from JSON or YAML file

It is also possible to specify the values from a local JSON or YAML file using flag `--replace-file` for the value and `--replace-path` for the path.

In which case the value encoding will be determined by the global flag `[ -e | --encoding ]`, both `JSON` and `JSON_IETF` are supported

The file is identified by its extension, json: `.json` and yaml `.yaml` or `.yml`.

=== "interface.json"
    ```bash
    {
        "admin-state": "enable",
        "ipv4": {
            "primary": {
                "address": "1.1.1.1",
                "prefix-length": 32
            }
        }
    }
    ```
=== "interface.yml"
    ```bash
    "admin-state": enable
    "ipv4":
    "primary":
        "address": 1.1.1.1
        "prefix-length": 32
    ```

Then refer to the file with `--replace-file` flag

``` bash
gnmic set --replace-path /configure/router[router-name=Base]/interface[interface-name=system] \
          --replace-file interface.json
```

## Delete Request

A deletion operation within the Set RPC is specified using the delete flag `--delete`.

It takes an XPATH pointing to the config node to be deleted:

```bash
gnmic set --delete "/configure/router[router-name=Base]/interface[interface-name=dummy_interface]"
```

## Templated Set Request file

A Set Request can also be built based on one or multiple templates and (optionally) a set of variables.

The variables allow to generate a Set Request file on per target basis.

If no variable file is found, the execution continues and the template is assumed to be a static string.

Each template specified with the flag `--request-file` is rendered against the variables defined in the file set with `--request-vars`.
Each template results in a single gNMI Set Request.

```bash
gnmic set --request-file <template1> --request-file <template2> --request-vars <vars_file>
```

### Template Format

The rendered template data can be a `JSON` or `YAML` valid string.

It has 3 sections, `updates`, `replaces` and `deletes`.

In each of the `updates` and `replaces`, a `path`, a `value` and an `encoding` can be configured.

If not specified, `path` defaults to `/`, while `encoding` defaults to the value set with `--encoding` flag.

`updates` and `replaces` result in a set of gNMI Set Updates in the Set RPC, `deletes` result in a set of gNMI paths to be deleted.

The `value` can be any arbitrary data format that the target accepts, it will be encoded based on the value of "encoding".
=== "JSON"
    ```json
    {
      "updates": [
          {
              "path": "/interface[name=ethernet-1/1]",
              "value": {
                  "admin-state": "enable",
                  "description": "to_spine1"
               },
               "encoding": "json_ietf"
          },
          {
              "path": "/interface[name=ethernet-1/2]",
              "value": {
                  "admin-state": "enable",
                  "description": "to_spine2"
               },
               "encoding": "json_ietf"
          }
      ],
      "replaces": [
          {
              "path": "/interface[name=ethernet-1/3]",
              "value": {
                  "admin-state": "enable",
                  "description": "to_spine3"
               }
          },
           {
              "path": "/interface[name=ethernet-1/4]",
              "value": {
                  "admin-state": "enable",
                  "description": "to_spine4"
               }
          }
      ],
      "deletes" : [
          "/interface[name=ethernet-1/5]",
          "/interface[name=ethernet-1/6]"
      ]
    }
    ```
=== "YAML"
    ```yaml
    updates:
      - path: "/interface[name=ethernet-1/1]"
        value:
          admin-state: enable
          description: "to_spine1"
        encoding: "json_ietf"
      - path: "/interface[name=ethernet-1/2]"
        value:
          admin-state: enable
          description: "to_spine2"
        encoding: "json_ietf"
    replaces:
      - path: "/interface[name=ethernet-1/3]"
        value:
          admin-state: enable
          description: "to_spine3"
      - path: "/interface[name=ethernet-1/4]"
        value:
          admin-state: enable
          description: "to_spine4"
    deletes:
      - "/interface[name=ethernet-1/5]"
      - "/interface[name=ethernet-1/6]"
    ```

### Per Target Template Variables

The file `--request-file` can be written as a [Go Text template](https://golang.org/pkg/text/template/).

The parsed template is loaded with additional functions from [gomplate](https://docs.gomplate.ca/).

`gnmic` generates one gNMI Set request per target.

The template will be rendered using variables read from the file `--request-vars`. 
Just like the template file, the variables file can either be a `JSON` or `YAML` formatted file.

If the flag `--request-vars` is not set, `gnmic` looks for a file with the same path, name and **extension** as the `request-file`, appended with `_vars`.

Within the template, the variables defined in the `--request-vars` file are accessible using the `.Vars` notation, while the target name is accessible using the `.TargetName` notation.

Example request template:

```yaml
replaces:
{{ $target := index .Vars .TargetName }}
{{- range $interface := index $target "interfaces" }}
  - path: "/interface[name={{ index $interface "name" }}]"
    encoding: "json_ietf"
    value: 
      admin-state: {{ index $interface "admin-state" | default "disable" }}
      description: {{ index $interface "description" | default "" }}
    {{- range $index, $subinterface := index $interface "subinterfaces" }}
      subinterface:
        - index: {{ $index }}
          admin-state: {{ index $subinterface "admin-state" | default "disable" }}
          {{- if has $subinterface "ipv4-address" }}
          ipv4:
            address:
              - ip-prefix: {{ index $subinterface "ipv4-address" | toString }}
          {{- end }}
          {{- if has $subinterface "ipv6-address" }}
          ipv6:
            address:
              - ip-prefix: {{ index $subinterface "ipv6-address" | toString }}
          {{- end }}
    {{- end }}
{{- end }}
```

The below variables file defines the input for 3 leafs:

```yaml
leaf1:57400:
  interfaces:
    - name: ethernet-1/1
      admin-state: "enable"
      description: "leaf1_to_spine1"
      subinterfaces:
        - admin-state: enable
          ipv4-address: 192.168.78.1/30
    - name: ethernet-1/2
      admin-state: "enable"
      description: "leaf1_to_spine2"
      subinterfaces:
        - admin-state: enable
          ipv4-address: 192.168.79.1/30

leaf2:57400:
  interfaces:
    - name: ethernet-1/1
      admin-state: "enable"
      description: "leaf2_to_spine1"
      subinterfaces:
        - admin-state: enable
          ipv4-address: 192.168.88.1/30
    - name: ethernet-1/2
      admin-state: "enable"
      description: "leaf2_to_spine2"
      subinterfaces:
        - admin-state: enable
          ipv4-address: 192.168.89.1/30
          
leaf3:57400:
  interfaces:
    - name: ethernet-1/1
      admin-state: "enable"
      description: "leaf3_to_spine1"
      subinterfaces:
        - admin-state: enable
          ipv4-address: 192.168.98.1/30
    - name: ethernet-1/2
      admin-state: "enable"
      description: "leaf3_to_spine2"
      subinterfaces:
        - admin-state: enable
          ipv4-address: 192.168.99.1/30
```

Result Request file per target:

=== "leaf1"
    ```yaml
    updates:
      - path: /interface[name=ethernet-1/1]
        encoding: "json_ietf"
        value: 
          admin-state: enable
          description: leaf1_to_spine1
          subinterface:
            - index: 0
              admin-state: enable
              ipv4:
                address:
                  - ip-prefix: 192.168.78.1/30
      - path: /interface[name=ethernet-1/2]
        encoding: "json_ietf"
        value: 
          admin-state: enable
          description: leaf1_to_spine2
          subinterface:
            - index: 0
              admin-state: enable
              ipv4:
                address:
                  - ip-prefix: 192.168.79.1/30
    ```
=== "leaf2"
    ```yaml
    updates:
      - path: /interface[name=ethernet-1/1]
        encoding: "json_ietf"
        value: 
          admin-state: enable
          description: leaf2_to_spine1
          subinterface:
            - index: 0
              admin-state: enable
              ipv4:
                address:
                  - ip-prefix: 192.168.88.1/30 
      - path: /interface[name=ethernet-1/2]
        encoding: "json_ietf"
        value: 
          admin-state: enable
          description: leaf2_to_spine2
          subinterface:
            - index: 0
              admin-state: enable
              ipv4:
                address:
                  - ip-prefix: 192.168.89.1/30 
    ```
=== "leaf3"
    ```yaml
    updates:
      - path: /interface[name=ethernet-1/1]
        encoding: "json_ietf"
        value: 
          admin-state: enable
          description: leaf3_to_spine1
          subinterface:
            - index: 0
              admin-state: enable
              ipv4:
                address:
                  - ip-prefix: 192.168.98.1/30 
      - path: /interface[name=ethernet-1/2]
        encoding: "json_ietf"
        value: 
          admin-state: enable
          description: leaf3_to_spine2
          subinterface:
            - index: 0
              admin-state: enable
              ipv4:
                address:
                  - ip-prefix: 192.168.99.1/30 
    ```

## Examples
#### 1. update
##### in-line value
```bash
gnmic -a <ip:port> set --update-path /configure/system/name \
                       --update-value <system_name>
```

##### value from JSON file
```bash
cat jsonFile.json
{"name": "router1"}

gnmic -a <ip:port> set --update-path /configure/system \
                       --update-file <jsonFile.json>
```

```bash
echo '{"name": "router1"}' | gnmic -a <ip:port> set \
                             --update-path /configure/system \
                             --update-file -
```

##### specify value type
```bash
gnmic -a <ip:port> set --update /configure/system/name:::json:::router1
gnmic -a <ip:port> set --update /configure/system/name@json@router1 \
                       --delimiter @
```

#### 2. replace
```bash
cat interface.json
{"address": "1.1.1.1", "prefix-length": 32}

gnmic -a <ip:port> --insecure \
      set --replace-path /configure/router[router-name=Base]/interface[interface-name=interface1]/ipv4/primary \
          --replace-file interface.json
```

```bash
echo '{"address": "1.1.1.1", "prefix-length": 32}' | gnmic -a <ip:port> --insecure \
      set --replace-path /configure/router[router-name=Base]/interface[interface-name=interface1]/ipv4/primary \
          --replace-file -
```

#### 3. delete
```bash
gnmic -a <ip:port> --insecure set --delete /configure/router[router-name=Base]/interface[interface-name=interface1]
```

<script
id="asciicast-319562" src="https://asciinema.org/a/319562.js" async>
</script>
