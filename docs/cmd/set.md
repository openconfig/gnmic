### Description
The `set` command represents the [gNMI Set RPC](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L62).

It is used to send a [Set Request](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L339) to the specified target(s) and expects one [Set Response](https://github.com/openconfig/gnmi/blob/master/proto/gnmi/gnmi.proto#L356) per target.

Set RPC allows the client to [modify the state](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#34-modifying-state) of data on the target. The data specified referenced by a path can be [updated, replaced or deleted](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#343-transactions).

### Usage
`gnmic [global-flags] set [local-flags]`

The Set Request can be any of (or a combination of) update, replace or/and delete operations.

### Common flags
#### prefix
The `--prefix` flag sets a common [prefix](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#241-path-prefixes) to all paths specified using the local `--path` flag. Defaults to `""`.


If a user needs to provide [origin](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#222-paths) information to the Path message, the following pattern should be used for the path string: `"origin:path"`:

!!! note
    The path after the origin value has to start with a `/`

```
gnmic set --update "openconfig-interfaces:/interfaces/interface:::<type>:::<value>"
```

#### target
With the optional `[--target]` flag it is possible to supply the [path target](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#2221-path-target) information in the prefix field of the SetRequest message.

### Update
There are several ways to perform an update operation with gNMI Set RPC:

#### 1. in-line update, implicit type
Using both `--update-path` and `--update-value` flags, a user can update a value for a given path.

!!!warning
    With in-line update method the values provided with `--update-value` flag are **always** set to JSON type. If you need to specify other type for the value (i.e. JSON_IETF), use the [explicit type](#2-in-line-update-explicit-type) method.

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

### Replace
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

### Delete
A deletion operation within the Set RPC is specified using the delete flag `--delete`.

It takes an XPATH pointing to the config node to be deleted:

```bash
gnmic set --delete "/configure/router[router-name=Base]/interface[interface-name=dummy_interface]"
```

### Examples
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

#### 3. delete
```bash
gnmic -a <ip:port> --insecure set --delete /configure/router[router-name=Base]/interface[interface-name=interface1]
```

<script
id="asciicast-319562" src="https://asciinema.org/a/319562.js" async>
</script>
