### Description
`gnmic` can be used in a "dial-out telemetry" mode by means of the `listen` command. In the dial-out mode:

* a network element is configured with the telemetry paths
* a network element initiates a connection towards the server/collector (`gnmic` acts as a server in that case)

!!! info
    Currently `gnmic` only implements the dial-out support for Nokia[^1] SR OS 20.5.r1+ routers.

### Usage

```bash
gnmic listen [global flags] [local flags]
```

### Flags

#### address

The address flag `[-a | --address]` tells `gnmic` which address to bind an internal server to in an `address:port` format, e.g.: `0.0.0.0:57400`.

#### tls-cert

Path to the TLS certificate can be supplied with `--tls-cert` flag.

#### tls-key

Path to the private key can be supplied with `--tls-key` flag.

#### max-concurrent-streams

To limit the maximum number of concurrent HTTP2 streams use the `--max-concurrent-streams` flag, the default value is 256.

### prometheus-address

The prometheus-address flag `[--prometheus-address]` allows starting a prometheus server that can be scraped by a prometheus client. It exposes metrics like memory, CPU and file descriptor usage.

### Examples

#### TLS disabled server

To start `gnmic` as a server listening on all interfaces without TLS support is as simple as:

```bash
gnmic listen -a 0.0.0.0:57400
```

??? info "SR OS configuration for non TLS dialout connections"
    ```
    /configure system telemetry destination-group "dialout" allow-unsecure-connection
    /configure system telemetry destination-group "dialout" destination 10.2.0.99 port 57400 router-instance "management"
    /configure system telemetry persistent-subscriptions { }
    /configure system telemetry persistent-subscriptions subscription "dialout" admin-state enable
    /configure system telemetry persistent-subscriptions subscription "dialout" sensor-group "port_stats"
    /configure system telemetry persistent-subscriptions subscription "dialout" mode sample
    /configure system telemetry persistent-subscriptions subscription "dialout" sample-interval 1000
    /configure system telemetry persistent-subscriptions subscription "dialout" destination-group "dialout"
    /configure system telemetry persistent-subscriptions subscription "dialout" encoding bytes
    /configure system telemetry sensor-groups { }
    /configure system telemetry sensor-groups { sensor-group "port_stats" }
    /configure system telemetry sensor-groups { sensor-group "port_stats" path "/state/port[port-id=1/1/c1/1]/statistics/in-octets" }
    ```

#### TLS enabled server

By using [tls-cert](#tls-cert) and [tls-key](#tls-key) flags it is possible to run `gnmic` with TLS.

```bash
gnmic listen -a 0.0.0.0:57400 --tls-cert gnmic.pem --tls-key gnmic-key.pem
```

??? info "SR OS configuration for a TLS enabled dialout connections"
    The configuration below does not utilise router-side certificates and uses the certificate provided by the server (gnmic). The router will also not verify the certificate.
    ```
    /configure system telemetry destination-group "dialout" tls-client-profile "client-tls"
    /configure system telemetry destination-group "dialout" destination 10.2.0.99 port 57400 router-instance "management"
    /configure system telemetry persistent-subscriptions { }
    /configure system telemetry persistent-subscriptions subscription "dialout" admin-state enable
    /configure system telemetry persistent-subscriptions subscription "dialout" sensor-group "port_stats"
    /configure system telemetry persistent-subscriptions subscription "dialout" mode sample
    /configure system telemetry persistent-subscriptions subscription "dialout" sample-interval 1000
    /configure system telemetry persistent-subscriptions subscription "dialout" destination-group "dialout"
    /configure system telemetry persistent-subscriptions subscription "dialout" encoding bytes
    /configure system telemetry sensor-groups { }
    /configure system telemetry sensor-groups { sensor-group "port_stats" }
    /configure system telemetry sensor-groups { sensor-group "port_stats" path "/state/port[port-id=1/1/c1/1]/statistics/in-octets" }

    /configure system security tls client-cipher-list "client-ciphers" { }
    /configure system security tls client-cipher-list "client-ciphers" cipher 1 name tls-rsa-with-aes128-cbc-sha
    /configure system security tls client-cipher-list "client-ciphers" cipher 2 name tls-rsa-with-aes128-cbc-sha256
    /configure system security tls client-cipher-list "client-ciphers" cipher 3 name tls-rsa-with-aes256-cbc-sha
    /configure system security tls client-cipher-list "client-ciphers" cipher 4 name tls-rsa-with-aes256-cbc-sha256
    
    /configure system security tls client-tls-profile "client-tls" admin-state enable
    /configure system security tls client-tls-profile "client-tls" cipher-list "client-ciphers"
    ```

[^1]: Nokia dial-out proto definition can be found in [karimra/sros-dialout](https://github.com/karimra/sros-dialout/blob/master/NOKIA-dial-out-telemetry.proto)
