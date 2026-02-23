// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"

	"github.com/openconfig/gnmic/pkg/api/utils"
)

// map of supported cipher suites
func ciphersMap() map[string]uint16 {
	return map[string]uint16{
		// secure, up to tls1.2
		"TLS_RSA_WITH_AES_128_CBC_SHA": tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		"TLS_RSA_WITH_AES_256_CBC_SHA": tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		// secure, only tls1.2
		"TLS_RSA_WITH_AES_128_GCM_SHA256": tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_RSA_WITH_AES_256_GCM_SHA384": tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		// secure, tls1.3
		"TLS_AES_128_GCM_SHA256":       tls.TLS_AES_128_GCM_SHA256,
		"TLS_AES_256_GCM_SHA384":       tls.TLS_AES_256_GCM_SHA384,
		"TLS_CHACHA20_POLY1305_SHA256": tls.TLS_CHACHA20_POLY1305_SHA256,
		// secure, ECDHE
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":          tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":          tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":       tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":       tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		// insecure
		"TLS_RSA_WITH_RC4_128_SHA":                tls.TLS_RSA_WITH_RC4_128_SHA,
		"TLS_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		"TLS_RSA_WITH_AES_128_CBC_SHA256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
		"TLS_ECDHE_RSA_WITH_RC4_128_SHA":          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
		"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":     tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	}
}

var cipherSuitesPreferenceOrder = []uint16{
	// AEADs w/ ECDHE
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,

	// CBC w/ ECDHE
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,

	// AEADs w/o ECDHE
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,

	// CBC w/o ECDHE
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,

	// 3DES
	tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,

	// disabled cipher suites
	// CBC_SHA256
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA256,

	// RC4
	tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	tls.TLS_RSA_WITH_RC4_128_SHA,
}

var disabledCipherSuites = []uint16{
	// CBC_SHA256
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA256,

	// RC4
	tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	tls.TLS_RSA_WITH_RC4_128_SHA,
}

var (
	defaultCipherSuitesLen = len(cipherSuitesPreferenceOrder) - len(disabledCipherSuites)
	defaultCipherSuites    = cipherSuitesPreferenceOrder[:defaultCipherSuitesLen]
)

var defaultCipherSuitesTLS13 = []uint16{
	tls.TLS_AES_128_GCM_SHA256,
	tls.TLS_AES_256_GCM_SHA384,
	tls.TLS_CHACHA20_POLY1305_SHA256,
}

// TargetConfig //
type TargetConfig struct {
	Name                       string            `mapstructure:"name,omitempty" yaml:"name,omitempty" json:"name,omitempty"`
	Address                    string            `mapstructure:"address,omitempty" yaml:"address,omitempty" json:"address,omitempty"`
	Username                   *string           `mapstructure:"username,omitempty" yaml:"username,omitempty" json:"username,omitempty"`
	Password                   *string           `mapstructure:"password,omitempty" yaml:"password,omitempty" json:"password,omitempty"`
	AuthScheme                 string            `mapstructure:"auth-scheme,omitempty" yaml:"auth-scheme,omitempty" json:"auth-scheme,omitempty"`
	Timeout                    time.Duration     `mapstructure:"timeout,omitempty" yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Insecure                   *bool             `mapstructure:"insecure,omitempty" yaml:"insecure,omitempty" json:"insecure,omitempty"`
	TLSCA                      *string           `mapstructure:"tls-ca,omitempty" yaml:"tls-ca,omitempty" json:"tls-ca,omitempty"`
	TLSCert                    *string           `mapstructure:"tls-cert,omitempty" yaml:"tls-cert,omitempty" json:"tls-cert,omitempty"`
	TLSKey                     *string           `mapstructure:"tls-key,omitempty" yaml:"tls-key,omitempty" json:"tls-key,omitempty"`
	SkipVerify                 *bool             `mapstructure:"skip-verify,omitempty" yaml:"skip-verify,omitempty" json:"skip-verify,omitempty"`
	TLSServerName              string            `mapstructure:"tls-server-name,omitempty" yaml:"tls-server-name,omitempty" json:"tls-server-name,omitempty"`
	Subscriptions              []string          `mapstructure:"subscriptions,omitempty" yaml:"subscriptions,omitempty" json:"subscriptions,omitempty"`
	Outputs                    []string          `mapstructure:"outputs,omitempty" yaml:"outputs,omitempty" json:"outputs,omitempty"`
	BufferSize                 uint              `mapstructure:"buffer-size,omitempty" yaml:"buffer-size,omitempty" json:"buffer-size,omitempty"`
	GRPCReadBufferSize         *int              `mapstructure:"grpc-read-buffer-size,omitempty" yaml:"grpc-read-buffer-size,omitempty" json:"grpc-read-buffer-size,omitempty"`
	GRPCWriteBufferSize        *int              `mapstructure:"grpc-write-buffer-size,omitempty" yaml:"grpc-write-buffer-size,omitempty" json:"grpc-write-buffer-size,omitempty"`
	GRPCConnWindowSize         *int              `mapstructure:"grpc-conn-window-size,omitempty" yaml:"grpc-conn-window-size,omitempty" json:"grpc-conn-window-size,omitempty"`
	GRPCWindowSize             *int              `mapstructure:"grpc-window-size,omitempty" yaml:"grpc-window-size,omitempty" json:"grpc-window-size,omitempty"`
	GRPCStaticConnWindowSize   *int              `mapstructure:"grpc-static-conn-window-size,omitempty" yaml:"grpc-static-conn-window-size,omitempty" json:"grpc-static-conn-window-size,omitempty"`
	GRPCStaticStreamWindowSize *int              `mapstructure:"grpc-static-stream-window-size,omitempty" yaml:"grpc-static-stream-window-size,omitempty" json:"grpc-static-stream-window-size,omitempty"`
	RetryTimer                 time.Duration     `mapstructure:"retry-timer,omitempty" yaml:"retry-timer,omitempty" json:"retry-timer,omitempty"`
	TLSMinVersion              string            `mapstructure:"tls-min-version,omitempty" yaml:"tls-min-version,omitempty" json:"tls-min-version,omitempty"`
	TLSMaxVersion              string            `mapstructure:"tls-max-version,omitempty" yaml:"tls-max-version,omitempty" json:"tls-max-version,omitempty"`
	TLSVersion                 string            `mapstructure:"tls-version,omitempty" yaml:"tls-version,omitempty" json:"tls-version,omitempty"`
	LogTLSSecret               *bool             `mapstructure:"log-tls-secret,omitempty" yaml:"log-tls-secret,omitempty" json:"log-tls-secret,omitempty"`
	ProtoFiles                 []string          `mapstructure:"proto-files,omitempty" yaml:"proto-files,omitempty" json:"proto-files,omitempty"`
	ProtoDirs                  []string          `mapstructure:"proto-dirs,omitempty" yaml:"proto-dirs,omitempty" json:"proto-dirs,omitempty"`
	Tags                       []string          `mapstructure:"tags,omitempty" yaml:"tags,omitempty" json:"tags,omitempty"`
	EventTags                  map[string]string `mapstructure:"event-tags,omitempty" yaml:"event-tags,omitempty" json:"event-tags,omitempty"`
	Gzip                       *bool             `mapstructure:"gzip,omitempty" yaml:"gzip,omitempty" json:"gzip,omitempty"`
	Token                      *string           `mapstructure:"token,omitempty" yaml:"token,omitempty" json:"token,omitempty"`
	Proxy                      string            `mapstructure:"proxy,omitempty" yaml:"proxy,omitempty" json:"proxy,omitempty"`
	//
	TunnelTargetType string            `mapstructure:"-" yaml:"tunnel-target-type,omitempty" json:"tunnel-target-type,omitempty"`
	Encoding         *string           `mapstructure:"encoding,omitempty" yaml:"encoding,omitempty" json:"encoding,omitempty"`
	Metadata         map[string]string `mapstructure:"metadata,omitempty" yaml:"metadata,omitempty" json:"metadata,omitempty"`
	CipherSuites     []string          `mapstructure:"cipher-suites,omitempty" yaml:"cipher-suites,omitempty" json:"cipher-suites,omitempty"`
	TCPKeepalive     time.Duration     `mapstructure:"tcp-keepalive,omitempty" yaml:"tcp-keepalive,omitempty" json:"tcp-keepalive,omitempty"`
	GRPCKeepalive    *ClientKeepalive  `mapstructure:"grpc-keepalive,omitempty" yaml:"grpc-keepalive,omitempty" json:"grpc-keepalive,omitempty"`

	tlsConfig *tls.Config
}

type ClientKeepalive struct {
	Time                time.Duration `mapstructure:"time,omitempty"`
	Timeout             time.Duration `mapstructure:"timeout,omitempty"`
	PermitWithoutStream bool          `mapstructure:"permit-without-stream,omitempty"`
}

func (tc TargetConfig) String() string {
	if tc.Password != nil {
		pwd := "****"
		tc.Password = &pwd
	}

	b, err := json.Marshal(tc)
	if err != nil {
		return ""
	}

	return string(b)
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func (tc *TargetConfig) DeepCopy() *TargetConfig {
	if tc == nil {
		return nil
	}
	ntc := &TargetConfig{
		Name:                       tc.Name,
		Address:                    tc.Address,
		Username:                   clonePtr(tc.Username),
		Password:                   clonePtr(tc.Password),
		AuthScheme:                 tc.AuthScheme,
		Timeout:                    tc.Timeout,
		Insecure:                   clonePtr(tc.Insecure),
		TLSCA:                      clonePtr(tc.TLSCA),
		TLSCert:                    clonePtr(tc.TLSCert),
		TLSKey:                     clonePtr(tc.TLSKey),
		SkipVerify:                 clonePtr(tc.SkipVerify),
		TLSServerName:              tc.TLSServerName,
		Subscriptions:              make([]string, 0, len(tc.Subscriptions)),
		Outputs:                    make([]string, 0, len(tc.Outputs)),
		BufferSize:                 tc.BufferSize,
		GRPCReadBufferSize:         clonePtr(tc.GRPCReadBufferSize),
		GRPCWriteBufferSize:        clonePtr(tc.GRPCWriteBufferSize),
		GRPCConnWindowSize:         clonePtr(tc.GRPCConnWindowSize),
		GRPCWindowSize:             clonePtr(tc.GRPCWindowSize),
		GRPCStaticConnWindowSize:   clonePtr(tc.GRPCStaticConnWindowSize),
		GRPCStaticStreamWindowSize: clonePtr(tc.GRPCStaticStreamWindowSize),
		RetryTimer:                 tc.RetryTimer,
		TLSMinVersion:              tc.TLSMinVersion,
		TLSMaxVersion:              tc.TLSMaxVersion,
		TLSVersion:                 tc.TLSVersion,
		LogTLSSecret:               clonePtr(tc.LogTLSSecret),
		ProtoFiles:                 make([]string, 0, len(tc.ProtoFiles)),
		ProtoDirs:                  make([]string, 0, len(tc.ProtoDirs)),
		Tags:                       make([]string, 0, len(tc.Tags)),
		EventTags:                  make(map[string]string, len(tc.EventTags)),
		Gzip:                       clonePtr(tc.Gzip),
		Token:                      clonePtr(tc.Token),
		Proxy:                      tc.Proxy,
		TunnelTargetType:           tc.TunnelTargetType,
		Encoding:                   clonePtr(tc.Encoding),
		Metadata:                   make(map[string]string, len(tc.Metadata)),
		CipherSuites:               make([]string, 0, len(tc.CipherSuites)),
		TCPKeepalive:               tc.TCPKeepalive,
	}
	ntc.Subscriptions = append(ntc.Subscriptions, tc.Subscriptions...)
	ntc.Outputs = append(ntc.Outputs, tc.Outputs...)
	ntc.ProtoFiles = append(ntc.ProtoFiles, tc.ProtoFiles...)
	ntc.ProtoDirs = append(ntc.ProtoDirs, tc.ProtoDirs...)
	ntc.Tags = append(ntc.Tags, tc.Tags...)
	ntc.CipherSuites = append(ntc.CipherSuites, tc.CipherSuites...)

	maps.Copy(ntc.EventTags, tc.EventTags)
	maps.Copy(ntc.Metadata, tc.Metadata)

	if tc.GRPCKeepalive != nil {
		ntc.GRPCKeepalive = &ClientKeepalive{
			Time:                tc.GRPCKeepalive.Time,
			Timeout:             tc.GRPCKeepalive.Timeout,
			PermitWithoutStream: tc.GRPCKeepalive.PermitWithoutStream,
		}
	}
	return ntc
}

func (tc *TargetConfig) SetTLSConfig(tlsConfig *tls.Config) {
	tc.tlsConfig = tlsConfig
}

// NewTLSConfig //
func (tc *TargetConfig) NewTLSConfig() (*tls.Config, error) {
	if tc.tlsConfig != nil {
		return tc.tlsConfig, nil
	}
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
	var skipVerify bool
	if tc.SkipVerify != nil {
		skipVerify = *tc.SkipVerify
	}
	tlsConfig, err := utils.NewTLSConfig(ca, cert, key, "", skipVerify, false)
	if err != nil {
		return nil, err
	}
	if tlsConfig == nil {
		return nil, nil
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
	tlsConfig.ServerName = tc.TLSServerName

	// tc.cipher-suites is not set
	if len(tlsConfig.CipherSuites) == 0 && len(tc.CipherSuites) == 0 {
		cs := make([]uint16, len(defaultCipherSuites), len(defaultCipherSuites)+len(defaultCipherSuitesTLS13))
		copy(cs, defaultCipherSuites)
		if tlsConfig.MaxVersion == tls.VersionTLS13 || tlsConfig.MaxVersion == 0 {
			cs = append(cs, defaultCipherSuitesTLS13...)
		}
		tlsConfig.CipherSuites = cs
	}
	// tc.cipher-suites is set
	if len(tlsConfig.CipherSuites) == 0 && len(tc.CipherSuites) != 0 {
		tlsConfig.CipherSuites = make([]uint16, 0, len(tc.CipherSuites))
		cmap := ciphersMap()
		for _, cs := range tc.CipherSuites {
			if _, ok := cmap[cs]; !ok {
				return nil, fmt.Errorf("unknown cipher suite %q", cs)
			}
			tlsConfig.CipherSuites = append(tlsConfig.CipherSuites, cmap[cs])
		}
	}
	return tlsConfig, nil
}

// GrpcDialOptions creates the grpc.dialOption list from the target's configuration
func (tc *TargetConfig) GrpcDialOptions() ([]grpc.DialOption, error) {
	tOpts := make([]grpc.DialOption, 0, 1)
	// gzip
	if tc.Gzip != nil && *tc.Gzip {
		tOpts = append(tOpts, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	}
	// gRPC keepalive
	if tc.GRPCKeepalive != nil {
		tOpts = append(tOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                tc.GRPCKeepalive.Time,
			Timeout:             tc.GRPCKeepalive.Timeout,
			PermitWithoutStream: tc.GRPCKeepalive.PermitWithoutStream,
		}))
	}

	if tc.GRPCReadBufferSize != nil {
		tOpts = append(tOpts,
			grpc.WithReadBufferSize(*tc.GRPCReadBufferSize))
	}

	if tc.GRPCWriteBufferSize != nil {
		tOpts = append(tOpts,
			grpc.WithWriteBufferSize(*tc.GRPCWriteBufferSize))
	}

	if tc.GRPCConnWindowSize != nil {
		tOpts = append(tOpts,
			grpc.WithInitialConnWindowSize(int32(*tc.GRPCConnWindowSize)))
	}

	if tc.GRPCWindowSize != nil {
		tOpts = append(tOpts,
			grpc.WithInitialWindowSize(int32(*tc.GRPCWindowSize)))
	}

	if tc.GRPCStaticConnWindowSize != nil {
		tOpts = append(tOpts,
			grpc.WithStaticConnWindowSize(int32(*tc.GRPCStaticConnWindowSize)))
	}

	if tc.GRPCStaticStreamWindowSize != nil {
		tOpts = append(tOpts,
			grpc.WithStaticStreamWindowSize(int32(*tc.GRPCStaticStreamWindowSize)))
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
	// token credentials
	if tc.Token != nil && *tc.Token != "" {
		tOpts = append(tOpts,
			grpc.WithPerRPCCredentials(
				oauth.TokenSource{
					TokenSource: oauth2.StaticTokenSource(
						&oauth2.Token{
							AccessToken: *tc.Token,
						},
					),
				},
			))
	}
	return tOpts, nil
}

func (tc *TargetConfig) UsernameString() string {
	if tc.Username == nil {
		return notApplicable
	}
	return *tc.Username
}

func (tc *TargetConfig) PasswordString() string {
	if tc.Password == nil {
		return notApplicable
	}
	return *tc.Password
}

func (tc *TargetConfig) InsecureString() string {
	if tc.Insecure == nil {
		return notApplicable
	}
	return fmt.Sprintf("%t", *tc.Insecure)
}

func (tc *TargetConfig) TLSCAString() string {
	if tc.TLSCA == nil || *tc.TLSCA == "" {
		return notApplicable
	}
	return *tc.TLSCA
}

func (tc *TargetConfig) TLSKeyString() string {
	if tc.TLSKey == nil || *tc.TLSKey == "" {
		return notApplicable
	}
	return *tc.TLSKey
}

func (tc *TargetConfig) TLSCertString() string {
	if tc.TLSCert == nil || *tc.TLSCert == "" {
		return notApplicable
	}
	return *tc.TLSCert
}

func (tc *TargetConfig) SkipVerifyString() string {
	if tc.SkipVerify == nil {
		return notApplicable
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

func (tc *TargetConfig) Equal(other *TargetConfig) bool {
	if tc == other {
		return true
	}
	if tc == nil || other == nil {
		return false
	}

	ptrEq := func(a, b any) bool {
		if a == nil && b == nil {
			return true
		}
		if a == nil || b == nil {
			return false
		}
		return reflect.DeepEqual(a, b)
	}

	return tc.Name == other.Name &&
		tc.Address == other.Address &&
		ptrEq(tc.Username, other.Username) &&
		ptrEq(tc.Password, other.Password) &&
		tc.AuthScheme == other.AuthScheme &&
		tc.Timeout == other.Timeout &&
		ptrEq(tc.Insecure, other.Insecure) &&
		ptrEq(tc.TLSCA, other.TLSCA) &&
		ptrEq(tc.TLSCert, other.TLSCert) &&
		ptrEq(tc.TLSKey, other.TLSKey) &&
		ptrEq(tc.SkipVerify, other.SkipVerify) &&
		tc.TLSServerName == other.TLSServerName &&
		slices.Equal(tc.Subscriptions, other.Subscriptions) &&
		slices.Equal(tc.Outputs, other.Outputs) &&
		tc.BufferSize == other.BufferSize &&
		tc.RetryTimer == other.RetryTimer &&
		tc.TLSMinVersion == other.TLSMinVersion &&
		tc.TLSMaxVersion == other.TLSMaxVersion &&
		tc.TLSVersion == other.TLSVersion &&
		ptrEq(tc.LogTLSSecret, other.LogTLSSecret) &&
		slices.Equal(tc.ProtoFiles, other.ProtoFiles) &&
		slices.Equal(tc.ProtoDirs, other.ProtoDirs) &&
		slices.Equal(tc.Tags, other.Tags) &&
		maps.Equal(tc.EventTags, other.EventTags) &&
		ptrEq(tc.Gzip, other.Gzip) &&
		ptrEq(tc.Token, other.Token) &&
		tc.Proxy == other.Proxy &&
		tc.TunnelTargetType == other.TunnelTargetType &&
		ptrEq(tc.Encoding, other.Encoding) &&
		maps.Equal(tc.Metadata, other.Metadata) &&
		slices.Equal(tc.CipherSuites, other.CipherSuites) &&
		tc.TCPKeepalive == other.TCPKeepalive &&
		reflect.DeepEqual(tc.GRPCKeepalive, other.GRPCKeepalive) &&
		tc.GRPCReadBufferSize == other.GRPCReadBufferSize &&
		tc.GRPCWriteBufferSize == other.GRPCWriteBufferSize &&
		tc.GRPCConnWindowSize == other.GRPCConnWindowSize &&
		tc.GRPCWindowSize == other.GRPCWindowSize &&
		tc.GRPCStaticConnWindowSize == other.GRPCStaticConnWindowSize &&
		tc.GRPCStaticStreamWindowSize == other.GRPCStaticStreamWindowSize
}
