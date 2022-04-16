The `event-data-convert` processor, converts data values matching one of the regular expressions from/to a specific data unit:

| Symbol | Unit    | Symbol | Unit      | Symbol  | Unit     |
| ------ | ------- | ------ | --------- | --------| -------- |
| `b`    | Bit     | `B`    | Byte      | `KiB`   | KibiByte |
| `kb`   | kiloBit | `KB`   | KiloByte  | `MiB`   | MebiByte |
| `mb`   | MegaBit | `MB`   | MegaByte  | `GiB`   | GibiByte |
| `gb`   | GigaBit | `GB`   | GigaByte  | `TiB`   | TebiByte |
| `tb`   | TeraBit | `TB`   | TeraByte  | `EiB`   | ExbiByte |
| `eb`   | ExaBit  | `EB`   | ExaByte   | `ZiB`   | ZebiByte |
|        |         | `ZB`   | ZetaByte  | `YiB`   | YobiByte |
|        |         | `YB`   | YottaByte |         |          |

The source values can be of any numeric type including a string with or without a unit, e.g: `2.3`, `1KB` or `1.1 TB`.

The unit of the original value can be derived as `Byte` from its name if it ends with `-bytes`, `-octets`, `_bytes` or `_octets`.

### Examples

#### simple conversion

The below processor will convert any value with a name ending in `-octets` from `Byte` to `KiloByte`.

```yaml
processors:
  # processor name
  convert-data-unit:
    # processor type
    event-data-convert:
      # list of regex to be matched with the values names
      value-names: 
        - ".*-octets$"
      # the source value unit, defaults to B (Byte)
      from: B
      # the desired value unit, defaults to B (Byte)
      to: KB
      # keep the original value, 
      # a new value name will be added with the converted value,
      # the new value name will be the original name with _$to as suffix 
      # if no regex renaming is defined using `old` and `new`
      keep: false
      # old, a regex to be used to rename the converted value
      old: 
      # new, the replacement string
      new:
      # debug, enables this processor logging
      debug: false
```

=== "Event format before"
    ```json
    {
      "name": "default",
      "timestamp": 1607290633806716620,
      "tags": {
        "port_port-id": "A/1",
        "source": "172.17.0.100:57400",
        "subscription-name": "default"
      },
      "values": {
        "/state/port/ethernet/statistics/in-octets": "2048"
      }
    }
    ```
=== "Event format after"
    ```json
    {
      "name": "default",
      "timestamp": 1607290633806716620,
      "tags": {
        "port_port-id": "A/1",
        "source": "172.17.0.100:57400",
        "subscription-name": "default"
      },
      "values": {
        "/state/port/ethernet/statistics/in-octets": 2
      }
    }
    ```

#### conversion with renaming

The below data convert processor converts any value with a name ending in `-octets` from Byte to Kilobyte.
It will retain the original value while renaming the new value name by replacing `-octets` with `-kilobytes`.

```yaml
processors:
  # processor name
  convert-data-unit:
    # processor type
    event-data-convert:
      # list of regex to be matched with the values names
      value-names: 
        - ".*-octets$"
      # the source value unit, defaults to B (Byte)
      from: B
      # the desired value unit, defaults to B (Byte)
      to: KB
      # keep the original value, 
      # a new value name will be added with the converted value,
      # the new value name will be the original name with _$to as suffix 
      # if no regex renaming is defined using `old` and `new`
      keep: true
      # old, a regex to be used to rename the converted value
      old: ^(\S+)-octets$
      # new, the replacement string
      new: ${1}-kilobytes
      # debug, enables this processor logging
      debug: false
```

=== "Event format before"
    ```json
    {
      "name": "default",
      "timestamp": 1607290633806716620,
      "tags": {
        "port_port-id": "A/1",
        "source": "172.17.0.100:57400",
        "subscription-name": "default"
      },
      "values": {
        "/state/port/ethernet/statistics/in-octets": "2048"
      }
    }
    ```
=== "Event format after"
    ```json
    {
      "name": "default",
      "timestamp": 1607290633806716620,
      "tags": {
        "port_port-id": "A/1",
        "source": "172.17.0.100:57400",
        "subscription-name": "default"
      },
      "values": {
        "/state/port/ethernet/statistics/in-octets": "2048"
        "/state/port/ethernet/statistics/in-kilobytes": 2
      }
    }
    ```