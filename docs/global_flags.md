### address

The address flag `[-a | --address]` is used to specify the target's gNMI server address in address:port format, for e.g: `192.168.113.11:57400`

Multiple target addresses can be specified, either as comma separated values:

```bash
gnmic --address 192.168.113.11:57400,192.168.113.12:57400 
```

or by using the `--address` flag multiple times:

```bash
gnmic -a 192.168.113.11:57400 --address 192.168.113.12:57400
```

### cluster-name

The `[--cluster-name]` flag is used to specify the cluster name the `gnmic` instance will join. 

The cluster name is used as part of the locked keys to share targets between multiple gnmic instances.

Defaults to `default-cluster`

### config

The `--config` flag specifies the location of a configuration file that `gnmic` will read. 

If not specified, gnmic searches for a file named `.gnmic` with extensions `yaml, yml, toml or json` in the following locations:

* `$PWD`
* `$HOME`
* `$XDG_CONFIG_HOME`
* `$XDG_CONFIG_HOME/gnmic`

### debug

The debug flag `[-d | --debug]` enables the printing of extra information when sending/receiving an RPC

### dir

A path to a directory which `gnmic` would recursively traverse in search for the additional YANG files which may be required by YANG files specified with `--file` to build the YANG tree.

Can also point to a single YANG file instead of a directory.

Multiple `--dir` flags can be supplied.

### encoding

The encoding flag `[-e | --encoding]` is used to specify the [gNMI encoding](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#23-structured-data-types) of the Update part of a [Notification](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#21-reusable-notification-message-format) message.

It is case insensitive and must be one of: JSON, BYTES, PROTO, ASCII, JSON_IETF

### exclude

The `--exclude` flag specifies the YANG module __names__ to be excluded from the tree generation when YANG modules names clash.

Multiple `--exclude` flags can be supplied.

### file

A path to a YANG file or a directory with YANG files which `gnmic` will use with prompt, generate and path commands.

Multiple `--file` flags can be supplied.

### format

Five output formats can be configured by means of the `--format` flag. `[proto, protojson, prototext, json, event]` The default format is `json`.

The `proto` format outputs the gnmi message as raw bytes, this value is not allowed when the output type is file (file system, stdout or stderr) see [outputs](user_guide/outputs/output_intro.md)

The `prototext` and `protojson` formats are the message representation as defined in [prototext](https://godoc.org/google.golang.org/protobuf/encoding/prototext) and [protojson](https://godoc.org/google.golang.org/protobuf/encoding/protojson)

The `event` format emits the received gNMI SubscribeResponse updates and deletes as a list of events tagged with the keys present in the subscribe path (as well as some metadata) and a timestamp

Here goes an example of the same response emitted to stdout in the respective formats:

=== "protojson"
    ```json
    {
      "update": {
      "timestamp": "1595584408456503938",
      "prefix": {
        "elem": [
          {
            "name": "state"
          },
          {
            "name": "system"
          },
          {
            "name": "version"
          }
        ]
      },
        "update": [
          {
            "path": {
              "elem": [
                {
                 "name": "version-string"
               }
              ]
            },
            "val": {
              "stringVal": "TiMOS-B-20.5.R1 both/x86_64 Nokia 7750 SR Copyright (c) 2000-2020 Nokia.\r\nAll rights reserved. All use subject to applicable license agreements.\r\nBuilt on Wed May 13 14:08:50 PDT 2020 by builder in /builds/c/205B/R1/panos/main/sros"
            }
          }
        ]
      }
    }
    ```

=== "prototext"
    ```yaml
    update: {
      timestamp: 1595584168675434221
      prefix: {
        elem: {
          name: "state"
        }
        elem: {
          name: "system"
        }
        elem: {
          name: "version"
        }
      }
      update: {
        path: {
          elem: {
            name: "version-string"
          }
        }
        val: {
          string_val: "TiMOS-B-20.5.R1 both/x86_64 Nokia 7750 SR Copyright (c) 2000-2020 Nokia.\r\nAll rights reserved. All use subject to applicable license agreements.\r\nBuilt on Wed May 13 14:08:50 PDT 2020 by builder in /builds/c/205B/R1/panos/main/sros"
        }
      }
    }
    ```
=== "json"
    ```json
    {
      "source": "172.17.0.100:57400",
      "subscription-name": "default",
      "timestamp": 1595584326775141151,
      "time": "2020-07-24T17:52:06.775141151+08:00",
      "prefix": "state/system/version",
      "updates": [
        {
          "Path": "version-string",
          "values": {
            "version-string": "TiMOS-B-20.5.R1 both/x86_64 Nokia 7750 SR Copyright (c) 2000-2020 Nokia.\r\nAll rights reserved. All use subject to applicable license agreements.\r\nBuilt on Wed May 13 14:08:50 PDT 2020 by builder in /builds/c/205B/R1/panos/main/sros"
          }
        }
      ]
    }
    ```
=== "event"
    ```json
    [
      {
        "name": "default",
        "timestamp": 1595584587725708234,
        "tags": {
          "source": "172.17.0.100:57400",
          "subscription-name": "default"
        },
        "values": {
          "/state/system/version/version-string": "TiMOS-B-20.5.R1 both/x86_64 Nokia 7750 SR Copyright (c) 2000-2020 Nokia.\r\nAll rights reserved. All use subject to applicable license agreements.\r\nBuilt on Wed May 13 14:08:50 PDT 2020 by builder in /builds/c/205B/R1/panos/main/sros"
        }
      }
    ]
    ```

### gzip

The `[--gzip]` flag enables gRPC gzip compression.

### insecure

The insecure flag `[--insecure]` is used to indicate that the client wishes to establish an non-TLS enabled gRPC connection.

To disable certificate validation in a TLS-enabled connection use [`skip-verify`](#skip-verify) flag.

### instance-name

The `[--instance-name]` flag is used to give a unique name to the running `gnmic` instance. This is useful when there are multiple instances of `gnmic` running at the same time, either for high-availability and/or scalability

### log

The `--log` flag enables log messages to appear on stderr output. By default logging is disabled.

### log-file

The log-file flag `[--log-file <path>]` sets the log output to a file referenced by the path. This flag supersede the `--log` flag

### no-prefix

The no prefix flag `[--no-prefix]` disables prefixing the json formatted responses with `[ip:port]` string.

Note that in case a single target is specified, the prefix is not added.

### password

The password flag `[-p | --password]` is used to specify the target password as part of the user credentials. If omitted, the password input prompt is used to provide the password.

Note that in case multiple targets are used, all should use the same credentials.

### proto-dir

The `[--proto-dir]` flag is used to specify a list of directories where `gnmic` will search for the proto file names specified with `--proto-file`.

### proto-file

The `[--proto-file]` flag is used to specify a list of proto file names that `gnmic` will use to decode ProtoBytes values. only Nokia SROS proto is currently supported.

### proxy-from-env

The proxy-from-env flag `[--proxy-from-env]` indicates that the gnmic should use the HTTP/HTTPS proxy addresses defined in the environment variables `http_proxy` and `https_proxy` to reach the targets specified using the `--address` flag.

### retry

The retry flag `[--retry] specifies the wait time before each retry.

Valid formats: 10s, 1m30s, 1h.  Defaults to 10s

### skip-verify

The skip verify flag `[--skip-verify]` indicates that the target should skip the signature verification steps, in case a secure connection is used.  

### targets-file

The `[--targets-file]` flag is used to configure a [file target loader](user_guide/target_discovery/file_discovery.md)

### timeout

The timeout flag `[--timeout]` specifies the gRPC timeout after which the connection attempt fails.

Valid formats: 10s, 1m30s, 1h.  Defaults to 10s

### tls-ca

The TLS CA flag `[--tls-ca]` specifies the root certificates for verifying server certificates encoded in PEM format.

### tls-cert

The tls cert flag `[--tls-cert]` specifies the public key for the client encoded in PEM format.

### tls-key

The tls key flag `[--tls-key]` specifies the private key for the client encoded in PEM format.

### tls-max-version

The tls max version flag `[--tls-max-version]` specifies the maximum supported TLS version supported by gNMIc when creating a secure gRPC connection.

### tls-min-version

The tls min version flag `[--tls-min-version]` specifies the minimum supported TLS version supported by gNMIc when creating a secure gRPC connection.

### tls-version

The tls version flag `[--tls-version]` specifies a single supported TLS version gNMIc when creating a secure gRPC connection.

This flag overwrites the previously listed flags `--tls-max-version` and `--tls-min-version`.

### log-tls-secret

The log TLS secret flag `[--log-tls-secret]` makes gnmic to log the per-session pre-master secret so that it can be used to [decrypt TLS](https://gitlab.com/wireshark/wireshark/-/wikis/TLS#tls-decryption) secured gNMI communications with, for example, Wireshark.

The secret will be saved to a file named `<target-name>.tlssecret.log`.

### token

The token flag `[--token]` sets a token value to be added to each RPC as an Authorization Bearer Token.

Applied only in the case of a secure gRPC connection.

### username

The username flag `[-u | --username]` is used to specify the target username as part of the user credentials. If omitted, the input prompt is used to provide the username.
