The `event-strings` processor, exposes a few of Golang strings transformation functions, there functions can be applied to tags, tag names, values or value names. 

Supported functions:

* `strings.Replace`
* `strings.TrimPrefix`
* `strings.TrimSuffix`
* `strings.Title`
* `strings.ToLower`
* `strings.ToUpper`
* `strings.Split`
* `filepath.Base`


```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-strings:
      value-names: []
      tag-names: []
      values: []
      tags: []
      transforms:
        # strings function name
        - replace:
            apply-on:  # apply the transformation on name or value
            keep: # keep the old value or not if the name changed
            old: # string to be replaced
            new: #replacement string of old
        - trim-prefix:
            apply-on: # apply the transformation on name or value
            prefix: # prefix to be trimmed
        - trim_suffix:
            apply-on: # apply the transformation on name or value
            suffix: # suffix to be trimmed
        - title:
            apply-on: # apply the transformation on name or value
        - to-upper:
            apply-on: # apply the transformation on name or value
        - to-lower:
            apply-on: # apply the transformation on name or value
        - split:
            apply-on: # apply the transformation on name or value
            split-on: # character to split on
            join-with: # character to join with
            ignore-first: # number of first items to ignore when joining
            ignore-last: # number of last items to ignore when joining
        - path-base:
            apply-on: # apply the transformation on name or value 
```
### Examples

#### replace

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-strings:
      value-names:
        - ".*"
      transforms:
        # strings function name
        - replace:
            apply-on: "name"
            old: "-"
            new: "_"
```

=== "Event format before"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "carrier-transitions": "1",
            "in-error-packets": "0",
            "in-fcs-error-packets": "0",
            "in-octets": "65382630",
            "in-unicast-packets": "107154",
            "out-error-packets": "0",
            "out-octets": "64721394",
            "out-unicast-packets": "105876"
        }
    }
    ```
=== "Event format after"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "carrier_transitions": "1",
            "in_error_packets": "0",
            "in_fcs_error_packets": "0",
            "in_octets": "65382630",
            "in_unicast_packets": "107154",
            "out_error_packets": "0",
            "out_octets": "64721394",
            "out_unicast_packets": "105876"
        }
    }
    ```

#### trim-prefix

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-strings:
      value-names:
        - ".*"
      transforms:
        # strings function name
        - trim-prefix:
            apply-on: "name"
            prefix: "/srl_nokia-interfaces:interface/statistics/"

```

=== "Event format before"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
            "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "3797",
            "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "288033",
            "/srl_nokia-interfaces:interface/statistics/in-octets": "65382630",
            "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "107154",
            "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "614",
            "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "11",
            "/srl_nokia-interfaces:interface/statistics/out-octets": "64721394",
            "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "105876"
        }
    }
    ```
=== "Event format after"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "carrier-transitions": "1",
            "in-broadcast-packets": "3797",
            "in-error-packets": "0",
            "in-fcs-error-packets": "0",
            "in-multicast-packets": "288033",
            "in-octets": "65382630",
            "in-unicast-packets": "107154",
            "out-broadcast-packets": "614",
            "out-error-packets": "0",
            "out-multicast-packets": "11",
            "out-octets": "64721394",
            "out-unicast-packets": "105876"
        }
    }
    ```

#### to-upper

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-strings:
      tag-names:
        - "interface_name"
        - "subscription-name"
      transforms:
        # strings function name
        - to-upper:
            apply-on: "value"

```

=== "Event format before"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
            "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "3797",
            "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "288033",
            "/srl_nokia-interfaces:interface/statistics/in-octets": "65382630",
            "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "107154",
            "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "614",
            "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "11",
            "/srl_nokia-interfaces:interface/statistics/out-octets": "64721394",
            "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "105876"
        }
    }
    ```
=== "Event format after"
    ```json
       {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "MGMT0",
            "source": "172.23.23.2:57400",
            "subscription-name": "DEFAULT"
        },
        "values": {
            "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
            "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "3797",
            "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "288033",
            "/srl_nokia-interfaces:interface/statistics/in-octets": "65382630",
            "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "107154",
            "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "614",
            "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "11",
            "/srl_nokia-interfaces:interface/statistics/out-octets": "64721394",
            "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "105876"
        }
    }
    ```
#### path-base

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-strings:
      value-names:
        - ".*"
      transforms:
        # strings function name
        - path-base:
            apply-on: "name"
```

=== "Event format before"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
            "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "3797",
            "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "288033",
            "/srl_nokia-interfaces:interface/statistics/in-octets": "65382630",
            "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "107154",
            "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "614",
            "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "11",
            "/srl_nokia-interfaces:interface/statistics/out-octets": "64721394",
            "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "105876"
        }
    }
    ```
=== "Event format after"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "carrier-transitions": "1",
            "in-broadcast-packets": "3797",
            "in-error-packets": "0",
            "in-fcs-error-packets": "0",
            "in-multicast-packets": "288033",
            "in-octets": "65382630",
            "in-unicast-packets": "107154",
            "out-broadcast-packets": "614",
            "out-error-packets": "0",
            "out-multicast-packets": "11",
            "out-octets": "64721394",
            "out-unicast-packets": "105876"
        }
    }
    ```

#### split

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-strings:
      value-names:
        - ".*"
      transforms:
        # strings function name
        - split:
            on: "name"
            split-on: "/"
            join-with: "_"
            ignore-first: 1

```

=== "Event format before"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
            "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "3797",
            "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "288033",
            "/srl_nokia-interfaces:interface/statistics/in-octets": "65382630",
            "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "107154",
            "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "614",
            "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "11",
            "/srl_nokia-interfaces:interface/statistics/out-octets": "64721394",
            "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "105876"
        }
    }
    ```
=== "Event format after"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "statistics_carrier-transitions": "1",
            "statistics_in-broadcast-packets": "3797",
            "statistics_in-error-packets": "0",
            "statistics_in-fcs-error-packets": "0",
            "statistics_in-multicast-packets": "288033",
            "statistics_in-octets": "65382630",
            "statistics_in-unicast-packets": "107154",
            "statistics_out-broadcast-packets": "614",
            "statistics_out-error-packets": "0",
            "statistics_out-multicast-packets": "11",
            "statistics_out-octets": "64721394",
            "statistics_out-unicast-packets": "105876"
        }
    }
    ```
#### multiple transforms


```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-strings:
      value-names:
        - ".*"
      transforms:
        # strings function name
        - path-base:
            apply-on: "name"
        - title:
            apply-on: "name"
        - replace:
            apply-on: "name"
            old: "-"
            new: "_"
```

=== "Event format before"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "/srl_nokia-interfaces:interface/statistics/carrier-transitions": "1",
            "/srl_nokia-interfaces:interface/statistics/in-broadcast-packets": "3797",
            "/srl_nokia-interfaces:interface/statistics/in-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-fcs-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/in-multicast-packets": "288033",
            "/srl_nokia-interfaces:interface/statistics/in-octets": "65382630",
            "/srl_nokia-interfaces:interface/statistics/in-unicast-packets": "107154",
            "/srl_nokia-interfaces:interface/statistics/out-broadcast-packets": "614",
            "/srl_nokia-interfaces:interface/statistics/out-error-packets": "0",
            "/srl_nokia-interfaces:interface/statistics/out-multicast-packets": "11",
            "/srl_nokia-interfaces:interface/statistics/out-octets": "64721394",
            "/srl_nokia-interfaces:interface/statistics/out-unicast-packets": "105876"
        }
    }
    ```
=== "Event format after"
    ```json
    {
        "name": "default",
        "timestamp": 1607291271894072397,
        "tags": {
            "interface_name": "mgmt0",
            "source": "172.23.23.2:57400",
            "subscription-name": "default"
        },
        "values": {
            "Carrier_transitions": "1",
            "In_broadcast_packets": "3797",
            "In_error_packets": "0",
            "In_fcs_error_packets": "0",
            "In_multicast_packets": "288033",
            "In_octets": "65382630",
            "In_unicast_packets": "107154",
            "Out_broadcast_packets": "614",
            "Out_error_packets": "0",
            "Out_multicast_packets": "11",
            "Out_octets": "64721394",
            "Out_unicast_packets": "105876"
        }
    }
    ```
