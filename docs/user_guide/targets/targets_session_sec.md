# Targets session security

In line with the guidelines detailed in the [gNMI Specification](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#31-session-security-authentication-and-rpc-authorization), it is mandatory to establish an encrypted TLS session between the client and the server. This measure is essential to ensure secure communication within the gNMI protocol.

```text
The session between the client and server MUST be encrypted using TLS - 
and a target or client MUST NOT fall back to unencrypted sessions. 
The target and client SHOULD implement TLS >= 1.2.
```

`gNMIc` provides the ability to tailor and modify the TLS session parameters of the gNMI client according to your specific requirements.

## TLS session types

When it comes to establishing a TLS session using `gNMIc`, various options are available to suit different use cases and environmental requirements. Whether it's a one-way TLS session, a session without certificate validation, or a mutual TLS (mTLS) session, each type caters to specific needs. The selection largely depends on the user's scenario and the degree of security and validation necessary. The upcoming sections will detail each of these session types, offering guidelines to aid in choosing the most appropriate for your specific requirements.

### Simple TLS session w/o server certificate validation

For scenarios requiring a simple TLS session without server certificate validation, such as in certain testing or development environments, you can use gNMIc's `--skip-verify` flag or the `skip-verify` attribute. This mode bypasses the typical certificate verification process and establishes a secure connection without validating the server's identity. Please exercise caution when using this feature, as it may expose the connection to potential security vulnerabilities. It is recommended primarily for non-production environments or controlled testing situations.

=== "cli"
    ```shell
    gnmic -a router1 --skip-verify \
                 get --path /interface/oper-state
    ```
=== "file"
    ```yaml
    targets:
      router1:
        address: router1
        skip-verify: true
    ```

### Simple TLS session with server certificate validation

When establishing a simple TLS session with server certificate validation for enhanced security, gNMIc offers the --tls-ca flag or the tls-ca attribute. These options allow you to point to a Certificate Authority (CA) certificate file. By doing so, the session not only ensures encrypted communication but also verifies the server's identity through its certificate. This validation process greatly enhances the security of the connection, ensuring the client is communicating with the intended server. It's an advisable setting for production environments where data security and integrity are crucial.

=== "cli"
    ```shell
    gnmic -a router1 --tls-ca ./ca.pem \
                    get --path /interface/oper-state
    ```
=== "file"
    ```yaml
    targets:
      router1:
        address: router1
        tls-ca: ./ca.pem
    ```

### Simple TLS session with server certificate validation and server name override

There are circumstances where the server's identity, as indicated by its certificate, doesn't match its expected hostname. For such scenarios, gNMIc enables the initiation of a simple TLS session with both server certificate validation and server name override. This functionality can be utilized by employing the `--tls-server-name` flag or the `tls-server-name` attribute.

By overriding the server name in the TLS session, users can specify a different hostname that matches the server's certificate, even if it's not the actual hostname of the server. This allows for successful validation and secure communication even in cases of server name discrepancies due to reasons like load balancing, proxying, etc...

This feature is particularly beneficial in complex network scenarios or during migrations, where server names might not yet align with their certificates. By ensuring both secure encrypted communication and flexible server name accommodation, it adds an extra layer of adaptability for secure communication, particularly in dynamic or complex network environments.

=== "cli"
    ```shell
    gnmic -a router1 --tls-ca ./ca.pem \
                    --tls-server-name server1 \
                    get --path /interface/oper-state
    ```
=== "file"
    ```yaml
    targets:
      router1:
        address: router1
        tls-ca: ./ca.pem
        tls-server-name: server1
    ```

### Mutual TLS (mTLS) session

For heightened security scenarios, gNMIc supports mutual TLS (mTLS) sessions. mTLS not only verifies the server's identity to the client, but also the client's identity to the server. This reciprocal verification is achieved using the --tls-cert and --tls-key flags, or the tls-cert and tls-key attributes. These options allow the user to specify a client certificate and client key, respectively.

By providing a client certificate (`--tls-cert` or `tls-cert` attribute) and a client key (`--tls-key` or `tls-key` attribute), gNMIc allows the server to confirm the identity of the client, ensuring that the client is legitimate and authorized to access the server resources.

Mutual TLS is particularly beneficial in use cases where both ends of a connection need to confirm the other's identity, providing a significantly higher level of trust and security. It reduces the risk of man-in-the-middle attacks and is especially valuable in environments where sensitive data is transmitted or strict access control is required.

=== "cli"
    ```shell
    gnmic -a router1 --tls-ca ./ca.pem \
                    --tls-cert ./router1.cert \
                    --tls-key ./router.key \
                    get --path /interface/oper-state
    ```
=== "file"
    ```yaml
    targets:
      router1:
        address: router1
        tls-ca: ./ca.pem
        tls-cert: ./router1.cert
        tls-key: ./router1.key
    ```

### mTLS session with server name override

=== "cli"
    ```shell
    gnmic -a router1 --tls-ca ./ca.pem \
                    --tls-server-name server1 \
                    --tls-cert ./router1.cert \
                    --tls-key ./router.key \
                    get --path /interface/oper-state
    ```
=== "file"
    ```yaml
    targets:
      router1:
        address: router1
        tls-ca: ./ca.pem
        tls-server-name: server1
        tls-cert: ./router1.cert
        tls-key: ./router1.key
    ```

## Configuring the client's TLS version

By default, `gNMIc` establishes a TLS session using the Golang's default TLS version (1.2), minimum version (1.2), and maximum version (1.3).

However, there might be scenarios where users need to control the TLS session negotiation to either test the server behavior or force the session into a specific version. To accommodate these needs, gNMIc provides flexibility by allowing users to explicitly set the TLS version.

Users can manipulate the negotiated TLS version using the flags (or target attributes) `--tls-version`, `--tls-min-version`, and `--tls-max-version`. These flags give control over the TLS session parameters, facilitating testing and customization of the communication session according to specific requirements.

Example: Forcing the client and server to use TLS1.3

=== "cli"
    ```shell
    gnmic -a router1 --tls-ca ./ca.pem \
                    --tls-cert ./router1.cert \
                    --tls-key ./router.key \
                    --tls-version 1.3 \
                    --tls-min-version 1.3 \
                    get --path /interface/oper-state
    ```
=== "file"
    ```yaml
    targets:
      router1:
        address: router1
        tls-ca: ./ca.pem
        tls-cert: ./router1.cert
        tls-key: ./router1.key
        tls-version: 1.3
        tls-min-version: 1.3
    ```

## Decrypting gNMI traffic using Wireshark

To facilitate advanced debugging or network analysis, gNMIc allows for the decryption of gNMI TLS traffic using the popular network protocol analyzer, Wireshark. The `--log-tls-secret` flag is instrumental in achieving this, as it stores the session pre-master secret, which can subsequently be used to decrypt TLS traffic.

When `--log-tls-secret` is used, the session's pre-master secret will be stored in a file named `<target-name>.tlssecret.log`. This secret enables Wireshark to decrypt the otherwise secure and encrypted TLS traffic between the client and the server.

Decryption of TLS traffic is particularly useful for network troubleshooting, performance optimization, or security audits. It allows network administrators or developers to deeply inspect packet data, diagnose network issues, and better understand data flows. However, this practice should be used carefully and ethically, given the sensitive nature of decrypted traffic, especially in production environments.

=== "cli"
    ```shell
    gnmic -a router1 --tls-ca ./ca.pem \
                    --log-tls-secret \
                    --tls-cert ./router1.cert \
                    --tls-key ./router.key \
                    get --path /interface/oper-state
    ```
=== "file"
    ```yaml
    targets:
      router1:
        address: router1
        tls-ca: ./ca.pem
        log-tls-secret: true
        tls-cert: ./router1.cert
        tls-key: ./router1.key
    ```
