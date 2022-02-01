
The package `github.com/karimra/gnmic/api` exposes a set of `api.TargetOption` that can be used with 
`api.NewTarget(...api.TargetOption) TargetOption` to create `target.Target`.

```golang
// Name sets the target name.
func Name(name string) TargetOption 

// Address sets the target address.
// This Option can be set multiple times.
func Address(addr string) TargetOption

// Username sets the target Username.
func Username(username string) TargetOption 

// Password sets the target Password.
func Password(password string) TargetOption 

// Timeout sets the gNMI client creation timeout.
func Timeout(timeout time.Duration) TargetOption

// Insecure sets the option to create a gNMI client with an
// insecure gRPC connection
func Insecure(i bool) TargetOption 

// SkipVerify sets the option to create a gNMI client with a
// secure gRPC connection without verifying the target's certificates.
func SkipVerify(i bool) TargetOption 

// TLSCA sets that path towards the TLS certificate authority file.
func TLSCA(tlsca string) TargetOption 

// TLSCert sets that path towards the TLS certificate file.
func TLSCert(cert string) TargetOption 

// TLSKey sets that path towards the TLS key file.
func TLSKey(key string) TargetOption 

// TLSMinVersion sets the TLS minimum version used during the TLS handshake.
func TLSMinVersion(v string) TargetOption 

// TLSMaxVersion sets the TLS maximum version used during the TLS handshake.
func TLSMaxVersion(v string) TargetOption

// TLSVersion sets the desired TLS version used during the TLS handshake.
func TLSVersion(v string) TargetOption 

// LogTLSSecret, if set to true,
// enables logging of the TLS master key.
func LogTLSSecret(b bool) TargetOption 

// Gzip, if set to true,
// adds gzip compression to the gRPC connection.
func Gzip(b bool) TargetOption 

// Token sets the per RPC credentials for all RPC calls. 
func Token(token string) TargetOption
```
