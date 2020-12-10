The `event_strings` processor, exposes a few of Golang strings transformation functions, there functions can be applied to tags, tag names, values or value names. 

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
event_processors:
  # processor name
  sample_processor:
    # processor type
    event_strings:
      value_names: []
      tag_names: []
      values: []
      tags: []
      transforms:
        # strings function name
        - replace:
            on:  # apply the transformation on name or value
            keep: # keep the old value or not if the name changed
            old: # string to be replaced
            new: #replacement string of old
        - trim_prefix:
            on: # apply the transformation on name or value
            prefix: # prefix to be trimmed
        - trim_suffix:
            on: # apply the transformation on name or value
            suffix: # suffix to be trimmed
        - title:
            on: # apply the transformation on name or value
        - to_upper:
            on: # apply the transformation on name or value
        - to_lower:
            on: # apply the transformation on name or value
        - split:
            on: # apply the transformation on name or value
            split_on: # charachter to split on
            join_with: # charachter to join with
            ignore_first: # number of first items to ignore when joining
            ignore_last: # number of last items to ignore when joining
        - path_base:
            on: # apply the transformation on name or value 
```
### Examples

#### replace

```yaml
event_processors:
  # processor name
  sample_processor:
    # processor type
    event_strings:
      value_names:
        - ".*"
      transforms:
        # strings function name
        - replace:
            on: "name"
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

#### trim_prefix

```yaml
event_processors:
  # processor name
  sample_processor:
    # processor type
    event_strings:
      value_names:
        - ".*"
      transforms:
        # strings function name
        - trim_prefix:
            on: "name"
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

#### to_upper

```yaml
event_processors:
  # processor name
  sample_processor:
    # processor type
    event_strings:
      tag_names:
        - "interface_name"
        - "subscription-name"
      transforms:
        # strings function name
        - to_upper:
            on: "value"

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
#### path_base

```yaml
event_processors:
  # processor name
  sample_processor:
    # processor type
    event_strings:
      value_names:
        - ".*"
      transforms:
        # strings function name
        - path_base:
            On: "name
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
event_processors:
  # processor name
  sample_processor:
    # processor type
    event_strings:
      value_names:
        - ".*"
      transforms:
        # strings function name
        - split:
            on: "name"
            split_on: "/"
            join_with: "_"
            ignore_first: 1

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