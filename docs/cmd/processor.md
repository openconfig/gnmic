### Description

The `[processor | proc]` command allows running a set of event processor offline given an input of event messages.

If expects a file input (`--input`) containing a list of event messages and one or more processor(s) name(s) (`--name`) defined in the main config file.
This command will read the input file, validate the configured processors, apply them on the input event messages and print out the result.

### Usage

`gnmic [global-flags] processor [local-flags]`

### Local Flags

The processor command supports the following local flags:

#### name

The `[--name]` flag sets the list of processors names to apply to the input.

#### input

The `[--input]` flag is used to specify the path to a file containing a list of event messages (`stdin` can be specified by giving the `-` value).

#### delimiter

The `[--delimiter]` flag is used to set the delimiter string between event messages in the input file, defaults to `\n`.

#### output

The `[--output]` flag references an output name configured in the main config file. The command will out format the resulting messages according to the output config. This is mainly for outputs with `type: prometheus`

### Example

Config File

```yaml
outputs:
  out1:
    type: prometheus
    metric-prefix: "gnmic"
    strings-as-labels: true
    
processors:
  proc0:
    event-strings:
      value-names:
        - "^_"
      transforms:

  # processor name
  proc1:
    # processor type
    event-strings:
      value-names:
        - ".*"
      transforms:
        # strings function name
        - path-base:
            apply-on: "name"
  proc2:
    event-strings:
      tag-names:
        - "interface_name"
        - "subscription-name"
        - "source"
      transforms:
        # strings function name
        - to-upper:
            apply-on: "value"
        - to-upper:
            apply-on: "name"
  proc3:
    # processor type
    event-drop:
      condition: ".values | length == 0"
```

input File:

```json
[
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/in-packets": 351770
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/in-octets": 35284165
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/in-unicast-packets": 338985
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/in-broadcast-packets": 1218
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/in-multicast-packets": 5062
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/in-discarded-packets": 6377
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/in-error-packets": 128
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/in-fcs-error-packets": 0
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/out-packets": 568218
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/out-octets": 219527024
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/out-mirror-octets": 0
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/out-unicast-packets": 567532
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/out-broadcast-packets": 6
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/out-multicast-packets": 680
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/out-discarded-packets": 0
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/out-error-packets": 0
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/out-mirror-packets": 0
    }
  },
  {
    "name": "sub1",
    "timestamp": 1710890476202665500,
    "tags": {
      "interface_name": "mgmt0",
      "source": "clab-traps-srl1",
      "subscription-name": "sub1"
    },
    "values": {
      "/interface/statistics/carrier-transitions": 1
    }
  }
]
```

Command:

```shell
gnmic processor --input /path/to/event_msg.txt --delimiter "\n###" --name proc1,proc2,proc3 --output out1
```

Output:

```text
# HELP gnmic_in_packets gNMIc generated metric
# TYPE gnmic_in_packets untyped
gnmic_in_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 351770
# HELP gnmic_in_octets gNMIc generated metric
# TYPE gnmic_in_octets untyped
gnmic_in_octets{subscription_name="sub1",interface_name="mgmt0",source="clab-traps-srl1"} 3.5284165e+07
# HELP gnmic_in_unicast_packets gNMIc generated metric
# TYPE gnmic_in_unicast_packets untyped
gnmic_in_unicast_packets{subscription_name="sub1",interface_name="mgmt0",source="clab-traps-srl1"} 338985
# HELP gnmic_in_broadcast_packets gNMIc generated metric
# TYPE gnmic_in_broadcast_packets untyped
gnmic_in_broadcast_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 1218
# HELP gnmic_in_multicast_packets gNMIc generated metric
# TYPE gnmic_in_multicast_packets untyped
gnmic_in_multicast_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 5062
# HELP gnmic_in_discarded_packets gNMIc generated metric
# TYPE gnmic_in_discarded_packets untyped
gnmic_in_discarded_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 6377
# HELP gnmic_in_error_packets gNMIc generated metric
# TYPE gnmic_in_error_packets untyped
gnmic_in_error_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 128
# HELP gnmic_in_fcs_error_packets gNMIc generated metric
# TYPE gnmic_in_fcs_error_packets untyped
gnmic_in_fcs_error_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 0
# HELP gnmic_out_packets gNMIc generated metric
# TYPE gnmic_out_packets untyped
gnmic_out_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 568218
# HELP gnmic_out_octets gNMIc generated metric
# TYPE gnmic_out_octets untyped
gnmic_out_octets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 2.19527024e+08
# HELP gnmic_out_mirror_octets gNMIc generated metric
# TYPE gnmic_out_mirror_octets untyped
gnmic_out_mirror_octets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 0
# HELP gnmic_out_unicast_packets gNMIc generated metric
# TYPE gnmic_out_unicast_packets untyped
gnmic_out_unicast_packets{subscription_name="sub1",interface_name="mgmt0",source="clab-traps-srl1"} 567532
# HELP gnmic_out_broadcast_packets gNMIc generated metric
# TYPE gnmic_out_broadcast_packets untyped
gnmic_out_broadcast_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 6
# HELP gnmic_out_multicast_packets gNMIc generated metric
# TYPE gnmic_out_multicast_packets untyped
gnmic_out_multicast_packets{source="clab-traps-srl1",subscription_name="sub1",interface_name="mgmt0"} 680
# HELP gnmic_out_discarded_packets gNMIc generated metric
# TYPE gnmic_out_discarded_packets untyped
gnmic_out_discarded_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 0
# HELP gnmic_out_error_packets gNMIc generated metric
# TYPE gnmic_out_error_packets untyped
gnmic_out_error_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 0
# HELP gnmic_out_mirror_packets gNMIc generated metric
# TYPE gnmic_out_mirror_packets untyped
gnmic_out_mirror_packets{interface_name="mgmt0",source="clab-traps-srl1",subscription_name="sub1"} 0
# HELP gnmic_carrier_transitions gNMIc generated metric
# TYPE gnmic_carrier_transitions untyped
gnmic_carrier_transitions{subscription_name="sub1",interface_name="mgmt0",source="clab-traps-srl1"} 1
```
