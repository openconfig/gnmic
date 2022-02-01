
The package `github.com/karimra/gnmic/api` exposes a set of `api.TargetOption` that can be used with 
`api.NewTarget(...api.TargetOption)` to create `target.Target`.

```golang
// Name sets the target name.
func Name(name string) 

// Address sets the target address.
func Address(addr string)

// Username sets the target Username.
func Username(username string) 

// Password sets the target Password.
func Password(password string) 

// Timeout sets the gNMI client creation timeout.
func Timeout(timeout time.Duration)

// Insecure if set to true,
// creates a gNMI client using an insecure gRPC connection.
func Insecure(i bool) 

// SkipVerify if set to true, 
// creates a gNMI client with a secure gRPC connection without verifying the target's certificates.
func SkipVerify(i bool) 

// TLSCA sets that path towards the TLS certificate authority file.
func TLSCA(tlsca string) 

// TLSCert sets that path towards the TLS certificate file.
func TLSCert(cert string) 

// TLSKey sets that path towards the TLS key file.
func TLSKey(key string) 

// TLSMinVersion sets the TLS minimum version used during the TLS handshake.
func TLSMinVersion(v string) 

// TLSMaxVersion sets the TLS maximum version used during the TLS handshake.
func TLSMaxVersion(v string)

// TLSVersion sets the desired TLS version used during the TLS handshake.
func TLSVersion(v string) 

// LogTLSSecret, if set to true,
// enables logging of the TLS master key.
func LogTLSSecret(b bool) 

// Gzip, if set to true,
// adds gzip compression to the gRPC connection.
func Gzip(b bool) 

// Token sets the per RPC credentials for all RPC calls. 
func Token(token string)
```
