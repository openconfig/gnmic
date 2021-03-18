The `event-trigger` processor, triggers an action if the configured condition evaluates to `true`.

The condition is evaluated using the [expr](https://github.com/antonmedv/expr) package with the event message as input.

Examples:

- The below expression checks if the value named `counter1` exists and has a value higher than 90
```bash
#     condition      ?          yes              : no
"counter1" in Values ? (Values["counter1"] > 90) : false
```

- This expression checks if the event name is `sub1` and that the tag `source` exists and is equal to `r1:57400`
```bash
Name == "sub1" and (("source" in Tags ? (Tags["source"] == "r1:57400")): false)
```

The trigger can be monitored over a configurable window of time (default 1 minute), during which only a certain number of occurrences (default 1) trigger the action.

Actions can be of two types:

- [HTTP action](#http-action): trigger an HTTP request
- [gNMI action](#gnmi-action): trigger a Get or Set gnmi RPC

### HTTP Action

Using the `HTTP action` you can send an HTTP request to a remote server, by default the whole event message is added to request body as a json payload.

The request body can be customized using [Go Templates](https://golang.org/pkg/text/template/) that take the event message as input.

The `HTTP action` templates come with some handy functions like:

- `withTags`: keep only certain tags in the event message.  
  for e.g: `{{ withTags . "tag1" "tag2" }}`
- `withoutTags`: remove certain tags from the event message.   
  for e.g: `{{ withoutTags . "tag1" "tag2" }}`
- `withValues`: keep only certain values in the event message.   
  for e.g: `{{ withValues . "counter1" "counter2" }}`
- `withoutTags`: remove certain values from the event message.   
  for e.g: `{{ withoutTags . "counter1" "counter2" }}`

```yaml
processors:
  # processor name
  my_trigger_proc: # 
    # processor type
    event-trigger:
      # trigger condition
      condition: '"counter1" in Values ? (Values["counter1"] > 90) : false'
      # number of condition occurrences before triggering the action
      max-occurrences: 2
      # window of time during which max-occurrences need to 
      # be reached in order to trigger the action
      window: 120s
      # the action to trigger
      action:
        # action type
        type: http
        # HTTP method
        method: POST
        # target url
        url: http://remote-server:p8080/
        # http headers to add to the request, this is a dictionary
        headers: 
          content-type: application/text
          # other-header: value
        # http request timeout
        timeout: 5s
        # go template used to build the request body.
        # if left empty the whole event message is added as a json object to the request's body
        body: '"counter1" crossed threshold {{ .Values["counter1"] }}'
        # enable extra logging
        debug: false
```

The below example triggers an HTTP GET to `http://remote-server:p8080/` if the value of counter "counter1" crosses 90 twice within 2 minutes.

```yaml
processors:
  my_trigger_proc:
    event-trigger:
      condition: '"counter1" in Values ? (Values["counter1"] > 90) : false'
      max-occurrences: 2
      window: 120s
      action:
        type: http
        method: POST
        url: http://remote-server:8080/
        headers: 
          content-type: application/text
        timeout: 5s
        body: '"counter1" crossed threshold, value={{ index .Values "counter1" }}'
```
### gNMI Action

Using the `gNMI action` you can trigger a gNMI Get or Set RPC when the trigger condition is met.

Just like the `HTTP action` the RPC fields can be customized using [Go Templates](https://golang.org/pkg/text/template/)

```yaml
processors:
  # processor name
  my_trigger_proc: # 
    # processor type
    event-trigger:
      # trigger condition
      condition: '"/interface/interface/oper-state" in Values ? (Values["/interface/interface/oper-state"] == "DOWN") : false'
      # number of condition occurrences before triggering the action
      max-occurrences: 2
      # window of time during which max-occurrences need to 
      # be reached in order to trigger the action
      window: 120s
      # the action to trigger
      action:
        # action type
        type: gnmi
        # gNMI rpc, defaults to `get`, if `set` is used it will default to a set update.
        # to trigger a set replace, use `set-replace`
        rpc: set
        # the target router, it defaults to the value in tag "source"
        target: {{ index .Tags "source" }}
        # paths templates to build xpaths
        paths:
          - | 
            {{ if eq ( index .Tags "interface_name" ) "ethernet-1/1"}}
              {{$interfaceName := "ethernet-1/2"}}
            {{else}}
              {{$interfaceName := "ethernet-1/1"}}
            {{end}}
            /interfaces/interface[name={{$interfaceName}}]/admin-state
        # values templates to build the values in case of set-update or set-replace
        values:
          - "enable"
        # data-type in case of get RPC, one of: ALL, CONFIG, STATE, OPERATIONAL
        data-type: ALL
        # gNMI encoding, defaults to json
        encoding: json
        # debug, enable extra logging
        debug: false
```

The below example shows a trigger that enables a router interface if another interface's operational status changes to "DOWN".

```yaml
processors:
  my_trigger_proc:
    event-trigger:
      condition: '"/interface/interface/oper-state" in Values ? (Values["/interface/interface/oper-state"] == "DOWN") : false'
      action:
        type: gnmi
        rpc: set-update
        target: {{ .Tags["source"] }}
        paths:
          - | 
            {{ if eq ( index .Tags "interface_name" ) "ethernet-1/1"}}
              {{$interfaceName := "ethernet-1/2"}}
            {{else}}
              {{$interfaceName := "ethernet-1/1"}}
            {{end}}
            /interfaces/interface[name={{$interfaceName}}]/admin-state
        values:
          - "enable"
```

