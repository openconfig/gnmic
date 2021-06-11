`gnmic` supports dynamic loading of gNMI targets from external systems.
This feature allows adding and deleting gNMI targets without the need to restart `gnmic`.

Depending on the discovery method, `gnmic` will either:

- Subscribe to changes on the remote system,
- Or poll the defined targets from the remote systems.
  
When a change is detected, the new targets are added and the corresponding subscriptions are immediately established.
The removed targets are deleted together with their subscriptions.

Three types of target discovery methods are supported:

- [File](./file_discovery.md): Watches changes to a local file containing gNMI targets definitions.
- [Consul Server](./consul_discovery.md): Subscribes to Consul KV key prefix changes, the keys and their value represent a target configuration fields
- [Docker Engine](./docker_discovery.md): Polls containers from a Docker Engine host matching some predefined criteria (docker filters).
  
!!! notes
    1. Only one discovery method is supported at a time.

    2. Target updates are not supported, delete and re-add is the way to update a target configuration.
