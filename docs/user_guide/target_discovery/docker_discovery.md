
The Docker target loader allows discovering gNMI targets from [Docker Engine](https://docs.docker.com/engine/) hosts.

It discovers containers as well as their gNMI address, based on a list of [Docker filters](https://docs.docker.com/engine/reference/commandline/ps/#filtering)

One gNMI target is added per discovered container.

Individual Target configurations are derived from the container exposed ports and labels, as well as the global configuration.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:3,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams/target_discovery.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2Ftarget_discovery.drawio" async></script>

#### Configuration

```yaml

loader:
  # the loader type: docker
  type: docker
  # string, the docker daemon address,
  # leave empty to use the local docker daemon
  # possible values:
  #  - unix:///var/run/docker.sock
  #  - tcp://<docker_host>:port
  #  - http://<docker_host>:port
  address: ""
  # duration, check interval for discovering 
  # new docker containers, default: 30s
  interval: 30s
  # duration, the docker queries timeout, 
  # defaults to half of `interval` if left unset or is invalid.
  timeout: 15s
  # time to wait before the fist docker query
  start-delay: 0s
  # bool, print loader debug statements.
  debug: false
  # if true, registers dockerLoader prometheus metrics with the provided
  # prometheus registry
  enable-metrics: false
  # containers, network filters: 
  # see https://docs.docker.com/engine/reference/commandline/ps/#filtering
  # for the possible values.
  filters:
      # containers filters
    - containers:
        # containers returned by `docker ps -f "label=clab-node-kind=srl"`
        - label: clab-node-kind=srl
      # network filters
      network:
        # networks returned by `docker network ls -f "label=containerlab"`
        label: containerlab
      # gNMI port value for the containers discovered by this filter.
      # It can be a port value or a label name set on the container.
      # valid values:
      #   `port: "57400"`
      #   `port: "label=gnmi-port"`
      port: 
      # target config for containers discovered by this filter.
      # These fields will override the matching global config fields.
      config:
        username: admin
        password: secret1
        skip-verify: true
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

##### Filter fields explanation

- **containers**: (Optional)
  
  A list of lists of docker filters used to select containers from the Docker Engine host.

  The docker filter `status=running` is implicitly added.
  
  If not set, all containers with `status=running` are selected.

- **network**: (Optional)

  A set of docker filters used to select the network to connect to the container.
  
  If not filter is set, all docker networks are considered.

- **port**: (Optional)

  This field is used to specify the gNMI port for the discovered containers.
  
  An integer can be specified in which case it will be used as the gNMI port for all discovered containers.
  
  Alternatively, a string in the format `label=<label_name>` can be set, where `<label_name>` is a docker label containing the gNMI port value.
  
  If no value is set, the global flag/value `port` is used.

- **config**: (Optional)

  A set of configuration parameters to be applied to all discovered targets by the container filter.

  The target config fields as defined [here](../targets.md#target-configuration-options) can be set, except `name` and `address` which are discovered by the loader.

#### Examples

##### Simple1

A simple docker loader with a single docker container filter.

It loads all containers deployed with [containerlab](https://containerlab.srlinux.dev/), in lab called `lab1`.

```yaml
loader:
  type: docker
  filters:
    - containers:
        - label: containerlab=lab1
```

In the above example, `gnmic` docker loader connects to the local Docker Daemon.

It will discover containers having label `containerlab=lab1` and add them as gNMI targets.

Default configuration applies to those added targets

##### Simple2

A simple docker loader with a single docker container filter.

It loads all containers deployed with [containerlab](https://containerlab.srlinux.dev/), having kind `srl`.

```yaml
loader:
  type: docker
  filters:
    - containers:
        - label: clab-node-kind=srl
```

In the above example, `gnmic` docker loader connects to the local Docker Daemon.

It will discover containers having label `clab-node-kind=srl` and add them as gNMI targets.

Default configuration applies to those added targets

##### Advanced Example

A more advanced docker loader, with 2 filers, custom networks, ports and target configuration.

```yaml
loader:
  type: docker
  address: unix:///var/run/docker.sock
  filters:
    # filter 1
    - containers:
        # containers returned by `docker ps -f "label=clab-node-kind=srl"`
        - label: clab-node-kind=srl
      network:
        # networks returned by `docker network ls -f "label=containerlab"`
        label: containerlab
      port: "57400"
      config:
        username: admin
        password: secret1
        skip-verify: true
    # filter 2
    - containers:
        # containers returned by `docker ps -f "label=clab-node-kind=ceos"`
        - label: clab-node-kind=ceos
        # containers returned by `docker ps -f "label=clab-node-kind=vr-sros"`
        - label: clab-node-kind=vr-sros
      network:
        # networks returned by `docker network ls -f "name=mgmt"`
        name: mgmt
      # the value of label=gnmi-port exported by each container`
      port: "label=gnmi-port"
      config:
        username: admin
        password: secret2
        insecure: true
```

In the above example, `gnmic` docker loader connects to the docker daemon using the local unix socket address.

It will discover 2 sets of containers matching 2 filters:

- Filter1:
    - Containers with label `clab-node-kind=srl`.
    - Use network with label `containerlab` to connect to them.
    - The port number is the same for all containers and is set to `57400`.
    - The config fields `username: admin`, `password: secret1` and `skip-verify: true` will be applied to all the containers discovered by this filter.

- Filter2:
    - Containers with labels `clab-node-kind-ceos` or `clab-node-vr-sros`
    - Use network with `name=mgmt` to connect to them. Note that Docker returns all networks with names containing `mgmt`
    - The port number is discovered from the label `gnmi-port` set on each container.
    - The config fields `username: admin`, `password: secret2` and `insecure: true` will be applied to all the containers discovered by this filter.
