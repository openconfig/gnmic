
Multiple instances of`gnmic` can be run in clustered mode in order to load share the targets connections and protect against failures.

The cluster mode allows `gnmic` to scale and be highly available at the same time

To join the cluster, the instances rely on a service discovery system and distributed KV store such as `Consul`,

### Clustering process

At startup, all instances belonging to a cluster:
  
* Enter an election process in order to become the cluster leader.
* Register their API service `gnmic-api` in a configured service discovery system.

Upon becoming the leader:

* The `gnmic` instance starts watching the registered `gnmic-api` services, 
and maintains a local cache of the active ones. These are essentially the instances restAPI addresses.
* The leader then waits for `clustering/leader-wait-timer` to allow the other instances to register their API services as well. 
This is useful in case an instance is slow to boot, which leaves it out of the initial load sharing process.
* The leader then enters a "target watch loop" (`clustering/targets-watch-timer`), 
at each iteration the leader tries to determine if all configured targets are handled by an instance of the cluster, 
this is done by checking if there is a lock maintained for each configured target.

The instances which failed to become the leader, continue to try to acquire the leader lock.
### Target distribution process

If the leader detects that a target does not have a lock, it triggers the target distribution process:

* Query all the targets keys from the KV store and calculate each instance load (number of maintained gNMI targets).
* If the target configuration includes `tags`, the leader selects the instance with the most matching tags (in order). 
If multiple instances have the same matching tags, the one with the lowest load is selected.
* If the target doesn't have configured tags, the leader simply select the least loaded instance to handle the target's subscriptions.
* Retrieve the selected instance API address from the local services cache.
* Send both the target configuration as well as a target activation action to the selected instance.
  
When a cluster instance gets assigned a target (target activation):

* Acquire a key lock for that specific target.
* Once the lock is acquired, create the configured gNMI subscriptions.
* Maintain the target lock for the duration of the gNMI subscription.

The whole target distribution process is repeated for each target missing a lock.

### Configuration

The cluster configuration is as simple as:

```yaml
# rest api address, format "address:port"
api: ""
# clustering related configuration fields
clustering:
  # the cluster name, tells with instances belong to the same cluster
  # it is used as part of the leader key lock, and the targets key locks
  # if no value is configured, the value from flag --cluster-name is used.
  # if the flag has the empty string as value, "default-cluster" is used.
  cluster-name: default-cluster
  # unique instance name within the cluster,
  # used as the value in the target locks,
  # used as the value in the leader lock.
  # if no value is configured, the value from flag --instance-name is used.
  # if the flag has the empty string as value, a value is generated in 
  # the format `gnmic-$UUID`
  instance-name: ""
  # service address to be registered in the locker(Consul)
  # if not defined, it defaults to the address part of the API address:port
  service-address: ""
  # gnmic instances API service watch timer
  # this is a long timer used by the cluster leader 
  # in a consul long-blocking query: 
  # https://www.consul.io/api-docs/features/blocking#implementation-details
  services-watch-timer: 60s
  # targets-watch-timer, targets watch timer, duration the leader waits 
  # between consecutive targets distributions
  targets-watch-timer: 20s
  # target-assignment-timeout, max time a leader waits for an instance to 
  # lock an assigned target.
  # if the timeout is reached the leader unassigns the target and reselects 
  # a different instance.
  target-assignment-timeout: 10s
  # leader wait timer, allows to configure a wait time after an instance
  # acquires the leader key.
  # this wait time goal is to give more chances to other instances to register 
  # their API services before the target distribution starts
  leader-wait-timer: 5s
  # ordered list of strings to be added as tags during api service 
  # registration in addition to `cluster-name=${cluster-name}` and 
  # `instance-name=${instance-name}`
  tags: []
  # locker is used to configure the KV store used for 
  # service registration, service discovery, leader election and targets locks
  locker:
    # type of locker, only consul is supported currently
    type: consul
    # address of the locker server
    address: localhost:8500
    # Consul Data center, defaults to dc1
    datacenter: 
    # Consul username, to be used as part of HTTP basicAuth
    username:
    # Consul password, to be used as part of HTTP basicAuth
    password:
    # Consul Token, is used to provide a per-request ACL token which overrides 
    # the agent's default token
    token:
    # session-ttl, session time-to-live after which a session is considered 
    # invalid if not renewed
    # upon session invalidation, all services and locks created using this session
    # are considered invalid.
    session-ttl: 10s
    # delay, a time duration (0s to 60s), in the event of  a session invalidation 
    # consul will prevent the lock from being acquired for this duration.
    # The purpose is to allow a gnmic instance to stop active subscriptions before 
    # another one takes over.
    delay: 5s
    # retry-timer, wait period between retries to acquire a lock 
    # in the event of client failure, key is already locked or lock lost.
    retry-timer: 2s
    # renew-period, session renew period, must be lower that session-ttl. 
    # if the value is greater or equal than session-ttl, is will be set to half 
    # of session-ttl.
    renew-period: 5s
    # debug, enable extra logging messages
    debug: false
```

A `gnmic` instance creates gNMI subscriptions only towards targets for which it acquired locks. It is also responsible for maintaining that lock for the duration of the subscription.


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams//locking.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2F/locking.drawio" async></script>

### Instance affinity

The target distribution process can be influenced using `tags` added to the target configuration.

By default, `gnmic` instances register their API service with 2 tags;

`cluster-name=${clustering/cluster-name}`
`instance-name=${clustering/instance-name}`

By adding the same tags to a target `router1` configuration (below YAML), the cluster leader will "assign" `router1` to instance `gnmic1` in cluster `my-cluster` regardless of the instance load.

```yaml
targets:
  router1:
    tags:
      - cluster-name=my-cluster
      - instance-name=gnmic1
```

Custom tags can be added to an instance API service registration in order to customize the instance affinity logic.

```yaml
clustering:
  tags:
    - my-custom-tag=value1
```

### Instance failure

In the event of an instance failure, its maintained targets locks expire, which on the next `clustering/targets-watch-timer` interval will be detected by the cluster leader.

The leader then performs the same target distribution process for those targets without a lock.

### Leader reelection

If a cluster leader fails, one of the other instances in the cluster eventually acquires the leader lock and becomes the cluster leader.

It then, proceeds with the targets distribution process to assign the unhandled targets to an instance in the cluster.

### Scalability

Using the same above-mentioned clustering mechanism, `gnmic` can horizontally scale the number of supported gNMI connections distributed across multiple `gnmic` instances.

The collected gNMI data can then be aggregated and made available through any of the running `gnmic` instances, regardless of whether that instance collected the data from the target or not.

The data aggregation is done by chaining `gnmic` [outputs](../user_guide/outputs/output_intro.md) and [inputs](../user_guide/inputs/input_intro.md) to build a gNMI data pipeline.

In the diagram below, the `gnmic` instances on the left and right side of NATS server can be identical.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:12,&quot;zoom&quot;:1.4,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/karimra/gnmic/diagrams/diagrams//scalability.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fkarimra%2Fgnmic%2Fdiagrams%2F/scalability.drawio" async></script>