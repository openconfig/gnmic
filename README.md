# gnmiClient
gnmi rpc capabilities, get, set, subscribe from the terminal

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
  set          run gnmi set on targets
  subscribe    subscribe to gnmi updates on targets
```
## Global flags
```
Flags:
  -a, --address strings   comma separated gnmi targets addresses
      --config string     config file (default is $HOME/.gnmiClient.yaml)
  -d, --debug             debug mode
  -e, --encoding string   one of: JSON, BYTES, PROTO, ASCII, JSON_IETF. (default "JSON")
  -h, --help              help for gnmiClient
      --insecure          insecure connection
  -p, --password string   password
      --skip-verify       skip verify tls connection
      --timeout string    grpc timeout (default "30s")
      --tls-ca string     tls certificate authority
      --tls-cert string   tls certificate
      --tls-key string    tls key
  -u, --username string   username
```
## Examples:
### 1. Capabilities request
####   - single host
```
$ gnmiClient -a <ip:port> capabilities --username <user> --password <password>
$ gnmiClient -a <ip:port> cap
```
####   - multiple hosts
```
$ gnmiClient -a <ip:port>,<ip:port> capabilities
$ gnmiClient -a <ip:port> -a <ip:port> cap
```

### 2. Get request
```
$ gnmiClient -a <ip:port> get --path /state/port[port-id=*]
$ gnmiClient -a <ip:port> get --path /state/port[port-id=*] --path /state/router[router-name=*]/interface[interface-name=*]
$ gnmiClient -a <ip:port> get --prefix /state --path port[port-id=*] --path router[router-name=*]/interface[interface-name=*]
```
### 3. Set Request
####  1. update
#####   - in-line value
```
$ gnmiClient -a <ip:port> set --update /configure/system/name --update-value <system_name>
```
#####   - json file value
```
$ gnmiClient -a <ip:port> set --update /configure/system --update-file <jsonFile.json>
$ cat jsonFile.json
{"name": "router1"}
```
####  2. replace
```
$ gnmiClient -a <ip:port> set --replace /configure/router[router-name=Base]/interface[interface-name=interface1]/ipv4/primary --replace-file interface.json
$ cat interface.json
{"address": "1.1.1.1", "prefix-length": 32}
```
####  3. delete
```
$ gnmiClient -a <ip:port> set --delete /configure/router[router-name=Base]/interface[interface-name=interface1]
```
### 4. Subscribe request
####  1. streaming, target-defined subscription
```
$ gnmiClient -a <ip:port> sub --path /state/port[port-id=*]/statistics
```
####  2. streaming, sample, 30s interval subscription
```
$ gnmiClient -a <ip:port> sub --path /state/port[port-id=*]/statistics --stream-subscription-mode sample --sampling-interval 30s
```
####  3. streaming, on-change, heartbeat interval 1min subscription
```
$ gnmiClient -a <ip:port> sub --path /state/port[port-id=*]/statistics --stream-subscription-mode on-change --heartbeat-interval 1m
```
####  4. once subscription
```
$ gnmiClient -a <ip:port> sub --path /state/port[port-id=*]/statistics --subscription-mode once
```
