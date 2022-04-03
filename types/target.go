package types

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/karimra/gnmic/utils"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/encoding/gzip"
)

// TargetConfig //
type TargetConfig struct {
	Name          string        `mapstructure:"name,omitempty" json:"name,omitempty" yaml:"name,omitempty"`
	Address       string        `mapstructure:"address,omitempty" json:"address,omitempty" yaml:"address,omitempty"`
	Username      *string       `mapstructure:"username,omitempty" json:"username,omitempty" yaml:"username,omitempty"`
	Password      *string       `mapstructure:"password,omitempty" json:"password,omitempty" yaml:"password,omitempty"`
	Timeout       time.Duration `mapstructure:"timeout,omitempty" json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Insecure      *bool         `mapstructure:"insecure,omitempty" json:"insecure,omitempty" yaml:"insecure,omitempty"`
	TLSCA         *string       `mapstructure:"tls-ca,omitempty" json:"tls-ca,omitempty" yaml:"tlsca,omitempty"`
	TLSCert       *string       `mapstructure:"tls-cert,omitempty" json:"tls-cert,omitempty" yaml:"tls-cert,omitempty"`
	TLSKey        *string       `mapstructure:"tls-key,omitempty" json:"tls-key,omitempty" yaml:"tls-key,omitempty"`
	SkipVerify    *bool         `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty" yaml:"skip-verify,omitempty"`
	Subscriptions []string      `mapstructure:"subscriptions,omitempty" json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
	Outputs       []string      `mapstructure:"outputs,omitempty" json:"outputs,omitempty" yaml:"outputs,omitempty"`
	BufferSize    uint          `mapstructure:"buffer-size,omitempty" json:"buffer-size,omitempty" yaml:"buffer-size,omitempty"`
	RetryTimer    time.Duration `mapstructure:"retry,omitempty" json:"retry-timer,omitempty" yaml:"retry-timer,omitempty"`
	TLSMinVersion string        `mapstructure:"tls-min-version,omitempty" json:"tls-min-version,omitempty" yaml:"tls-min-version,omitempty"`
	TLSMaxVersion string        `mapstructure:"tls-max-version,omitempty" json:"tls-max-version,omitempty" yaml:"tls-max-version,omitempty"`
	TLSVersion    string        `mapstructure:"tls-version,omitempty" json:"tls-version,omitempty" yaml:"tls-version,omitempty"`
	LogTLSSecret  *bool         `mapstructure:"log-tls-secret,omitempty" json:"log-tls-secret,omitempty" yaml:"log-tls-secret,omitempty"`
	ProtoFiles    []string      `mapstructure:"proto-files,omitempty" json:"proto-files,omitempty" yaml:"proto-files,omitempty"`
	ProtoDirs     []string      `mapstructure:"proto-dirs,omitempty" json:"proto-dirs,omitempty" yaml:"proto-dirs,omitempty"`
	Tags          []string      `mapstructure:"tags,omitempty" json:"tags,omitempty" yaml:"tags,omitempty"`
	Gzip          *bool         `mapstructure:"gzip,omitempty" json:"gzip,omitempty" yaml:"gzip,omitempty"`
	Token         *string       `mapstructure:"token,omitempty" json:"token,omitempty" yaml:"token,omitempty"`
	//
	TunnelTargetType string `mapstructure:"-" json:"tunnel-target-type,omitempty" yaml:"tunnel-target-type,omitempty"`
}

func (tc *TargetConfig) String() string {
	b, err := json.Marshal(tc)
	if err != nil {
		return ""
	}
	return string(b)
}

// NewTLSConfig //
func (tc *TargetConfig) NewTLSConfig() (*tls.Config, error) {
	var ca, cert, key string
	if tc.TLSCA != nil {
		ca = *tc.TLSCA
	}
	if tc.TLSCert != nil {
		cert = *tc.TLSCert
	}
	if tc.TLSKey != nil {
		key = *tc.TLSKey
	}
	tlsConfig, err := utils.NewTLSConfig(ca, cert, key, *tc.SkipVerify, false)
	if err != nil {
		return nil, err
	}

	if tc.LogTLSSecret != nil && *tc.LogTLSSecret {
		logPath := tc.Name + ".tlssecret.log"
		w, err := os.Create(logPath)
		if err != nil {
			return nil, err
		}
		tlsConfig.KeyLogWriter = w
	}

	tlsConfig.MaxVersion = tc.getTLSMaxVersion()
	tlsConfig.MinVersion = tc.getTLSMinVersion()
	return tlsConfig, nil
}

// GrpcDialOptions creates the grpc.dialOption list from teh target's configuration
func (tc *TargetConfig) GrpcDialOptions() ([]grpc.DialOption, error) {
	tOpts := make([]grpc.DialOption, 0, 1)
	// gzip
	if tc.Gzip != nil && *tc.Gzip {
		tOpts = append(tOpts, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	}
	// insecure
	if tc.Insecure != nil && *tc.Insecure {
		tOpts = append(tOpts,
			grpc.WithTransportCredentials(
				insecure.NewCredentials(),
			),
		)
		return tOpts, nil
	}
	// secure
	tlsConfig, err := tc.NewTLSConfig()
	if err != nil {
		return nil, err
	}
	tOpts = append(tOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if tc.Token != nil && *tc.Token != "" {
		tOpts = append(tOpts,
			grpc.WithPerRPCCredentials(
				oauth.NewOauthAccess(&oauth2.Token{
					AccessToken: *tc.Token,
				})))
	}

	return tOpts, nil
}

func (tc *TargetConfig) UsernameString() string {
	if tc.Username == nil {
		return "NA"
	}
	return *tc.Username
}

func (tc *TargetConfig) PasswordString() string {
	if tc.Password == nil {
		return "NA"
	}
	return *tc.Password
}

func (tc *TargetConfig) InsecureString() string {
	if tc.Insecure == nil {
		return "NA"
	}
	return fmt.Sprintf("%t", *tc.Insecure)
}

func (tc *TargetConfig) TLSCAString() string {
	if tc.TLSCA == nil || *tc.TLSCA == "" {
		return "NA"
	}
	return *tc.TLSCA
}

func (tc *TargetConfig) TLSKeyString() string {
	if tc.TLSKey == nil || *tc.TLSKey == "" {
		return "NA"
	}
	return *tc.TLSKey
}

func (tc *TargetConfig) TLSCertString() string {
	if tc.TLSCert == nil || *tc.TLSCert == "" {
		return "NA"
	}
	return *tc.TLSCert
}

func (tc *TargetConfig) SkipVerifyString() string {
	if tc.SkipVerify == nil {
		return "NA"
	}
	return fmt.Sprintf("%t", *tc.SkipVerify)
}

func (tc *TargetConfig) SubscriptionString() string {
	return fmt.Sprintf("- %s", strings.Join(tc.Subscriptions, "\n"))
}

func (tc *TargetConfig) OutputsString() string {
	return strings.Join(tc.Outputs, "\n")
}

func (tc *TargetConfig) BufferSizeString() string {
	return fmt.Sprintf("%d", tc.BufferSize)
}

func (tc *TargetConfig) getTLSMinVersion() uint16 {
	v := tlsVersionStringToUint(tc.TLSVersion)
	if v > 0 {
		return v
	}
	return tlsVersionStringToUint(tc.TLSMinVersion)
}

func (tc *TargetConfig) getTLSMaxVersion() uint16 {
	v := tlsVersionStringToUint(tc.TLSVersion)
	if v > 0 {
		return v
	}
	return tlsVersionStringToUint(tc.TLSMaxVersion)
}

func tlsVersionStringToUint(v string) uint16 {
	switch v {
	default:
		return 0
	case "1.3":
		return tls.VersionTLS13
	case "1.2":
		return tls.VersionTLS12
	case "1.1":
		return tls.VersionTLS11
	case "1.0", "1":
		return tls.VersionTLS10
	}
}
