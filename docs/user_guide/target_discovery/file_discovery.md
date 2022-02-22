
`gnmic` is able to watch changes happening to a file that contains the gNMI targets configuration.

The file can be located in the local file system or a remote one.

In case of remote file, `ftp`, `sftp`, `http(s)` protocols are supported.
The read timeout of remote files is set to half of the read `interval`

Newly added targets are discovered and subscribed to.
Deleted targets are moved from gNMIc's list and their subscriptions are terminated.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/target_discovery.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Ftarget_discovery.drawio" async></script>

#### Configuration

A file target loader can be configured in a couple of ways:

- using the `--targets-file` flag:

``` bash
gnmic --targets-file ./targets-config.yaml subscribe
```

``` bash
gnmic --targets-file sftp://user:pass@server.com/path/to/targets-file.yaml subscribe
```

- using the main configuration file:
  
``` yaml
loader:
  type: file
  # path to the file
  path: ./targets-config.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
  # time to wait before the fist file read
  start-delay: 0s
  # if true, registers fileLoader prometheus metrics with the provided
  # prometheus registry
  enable-metrics: false
  # list of actions to run on target discovery
  on-add:
  # list of actions to run on target removal
  on-delete:
  # variable dict to pass to actions to be run
  vars:
  # path to variable file, the variables defined will be passed to the actions to be run
  # values in this file will be overwritten by the ones defined in `vars`
  vars-file:
```

The `--targets-file` flag takes precedence over the `loader` configuration section.

The targets file can be either a `YAML` or a `JSON` file (identified by its extension json, yaml or yml), and follows the same format as the main configuration file `targets` section.
See [here](../../user_guide/targets.md#target-option)

### Examples

#### Local File
``` yaml
loader:
  type: file
  # path to the file
  path: ./targets-config.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
  # if true, registers fileLoader prometheus metrics with the provided
  # prometheus registry
  enable-metrics: false
```

#### Remote File

SFTP remote file

``` yaml
loader:
  type: file
  # path to the file
  path: sftp://user:pass@server.com/path/to/targets-file.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
  # if true, registers fileLoader prometheus metrics with the provided
  # prometheus registry
  enable-metrics: false
```

FTP remote file

``` yaml
loader:
  type: file
  # path to the file
  path: ftp://user:pass@server.com/path/to/targets-file.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
  # if true, registers fileLoader prometheus metrics with the provided
  # prometheus registry
  enable-metrics: false
```

HTTP remote file

``` yaml
loader:
  type: file
  # path to the file
  path: http://user:pass@server.com/path/to/targets-file.yaml
  # watch interval at which the file
  # is read again to determine if a target was added or deleted.
  interval: 30s
  # if true, registers fileLoader prometheus metrics with the provided
  # prometheus registry
  enable-metrics: false
```

#### Targets file format

=== "YAML"
    ```yaml
    10.10.10.10:
        username: admin
        insecure: true
    10.10.10.11:
        username: admin
    10.10.10.12:
    10.10.10.13:
    10.10.10.14:
    ```
=== "JSON"
    ```json
    {
        "10.10.10.10": {
            "username": "admin",
            "insecure": true
        },
         "10.10.10.11": {
            "username": "admin",
        },
         "10.10.10.12": {},
         "10.10.10.13": {},
         "10.10.10.14": {}
    }
    ```

Just like the targets in the main configuration file, the missing configuration fields get filled with the global flags,
the ENV variables first, the config file main section next and then the default values.
