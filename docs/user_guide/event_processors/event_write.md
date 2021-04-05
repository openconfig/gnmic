The `event-write` processor,  writes a message that has a value or a tag matching one of the configured regular expressions to `stdout`, `stderr` or to a file. 
A custom separator (used between written messages) can be configured, it defaults to `\n`

```yaml
processors:
  # processor name
  write-processor:
    # processor type
    event-write:
      # jq expression, if evaluated to true, the message is written to dst
      condition: 
      # list of regular expressions to be matched against the tags names, if matched, the message is written to dst
      tag-names:
      # list of regular expressions to be matched against the tags values, if matched, the message is written to dst
      tags:
      # list of regular expressions to be matched against the values names, if matched, the message is written to dst
      value-names:
      # list of regular expressions to be matched against the values, if matched, the message is written to dst
      values:
      # path to the destination file
      dst:
      # separator to be written between messages
      separator: 
      # indent to use when marshaling the event message to json
      indent:
```

### Examples
```yaml
processors:
  # processor name
  write-processor:
    # processor type
    event-write:
      value-names:
        - "."
      dst: file.log
      separator: "\n####\n"
      indent: "  "
```


``` bash
$ cat file.log
{
  "name": "sub1",
  "timestamp": 1607582483868459381,
  "tags": {
    "interface_name": "ethernet-1/1",
    "source": "172.20.20.5:57400",
    "subscription-name": "sub1"
  },
  "values": {
    "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
    "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "22",
    "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
    "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
    "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "8694",
    "/srl_nokia-interfaces:interface/statistics/in-octets": "1740350",
    "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "17",
    "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "22",
    "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
    "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "8696",
    "/srl_nokia-interfaces:interface/statistics/out-octets": "1723262",
    "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "17"
  }
}
####
{
  "name": "sub1",
  "timestamp": 1607582483868459381,
  "tags": {
    "interface_name": "ethernet-1/1",
    "source": "172.20.20.5:57400",
    "subscription-name": "sub1"
  },
  "values": {
    "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
    "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "22",
    "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
    "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
    "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "8694",
    "/srl_nokia-interfaces:interface/statistics/in-octets": "1740350",
    "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "17",
    "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "22",
    "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
    "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "8696",
    "/srl_nokia-interfaces:interface/statistics/out-octets": "1723262",
    "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "17"
  }
}
####
```