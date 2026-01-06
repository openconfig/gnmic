// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/adrg/xdg"
	"github.com/itchyny/gojq"
	"github.com/mitchellh/go-homedir"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
	yaml "gopkg.in/yaml.v2"

	"github.com/openconfig/gnmic/pkg/api"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	gfile "github.com/openconfig/gnmic/pkg/file"
	"github.com/zestor-dev/zestor/store"
)

const (
	configName      = ".gnmic"
	configLogPrefix = "[config] "
	envPrefix       = "GNMIC"
	trimChars       = " \r\n\t"
)

var ErrInvalidConfig = errors.New("invalid configuration")

var osPathFlags = []string{"tls-ca", "tls-cert", "tls-key"}

type Config struct {
	GlobalFlags `mapstructure:",squash"`
	LocalFlags  `mapstructure:",squash"`
	FileConfig  *viper.Viper `mapstructure:"-" json:"-" yaml:"-" `

	Targets       map[string]*types.TargetConfig       `mapstructure:"targets,omitempty" json:"targets,omitempty" yaml:"targets,omitempty"`
	Subscriptions map[string]*types.SubscriptionConfig `mapstructure:"subscriptions,omitempty" json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
	Outputs       map[string]map[string]any            `mapstructure:"outputs,omitempty" json:"outputs,omitempty" yaml:"outputs,omitempty"`
	Inputs        map[string]map[string]any            `mapstructure:"inputs,omitempty" json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Processors    map[string]map[string]any            `mapstructure:"processors,omitempty" json:"processors,omitempty" yaml:"processors,omitempty"`
	Clustering    *Clustering                          `mapstructure:"clustering,omitempty" json:"clustering,omitempty" yaml:"clustering,omitempty"`
	GnmiServer    *GNMIServer                          `mapstructure:"gnmi-server,omitempty" json:"gnmi-server,omitempty" yaml:"gnmi-server,omitempty"`
	APIServer     *APIServer                           `mapstructure:"api-server,omitempty" json:"api-server,omitempty" yaml:"api-server,omitempty"`
	Loader        map[string]any                       `mapstructure:"loader,omitempty" json:"loader,omitempty" yaml:"loader,omitempty"`
	Actions       map[string]map[string]any            `mapstructure:"actions,omitempty" json:"actions,omitempty" yaml:"actions,omitempty"`
	TunnelServer  *TunnelServer                        `mapstructure:"tunnel-server,omitempty" json:"tunnel-server,omitempty" yaml:"tunnel-server,omitempty"`
	//
	logger             *log.Logger
	setRequestTemplate []*template.Template
	setRequestVars     map[string]any
}

var ValueTypes = []string{"json", "json_ietf", "string", "int", "uint", "bool", "decimal", "float", "bytes", "ascii"}

type GlobalFlags struct {
	CfgFile       string
	Address       []string      `mapstructure:"address,omitempty" json:"address,omitempty" yaml:"address,omitempty"`
	Username      string        `mapstructure:"username,omitempty" json:"username,omitempty" yaml:"username,omitempty"`
	Password      string        `mapstructure:"password,omitempty" json:"password,omitempty" yaml:"password,omitempty"`
	Port          string        `mapstructure:"port,omitempty" json:"port,omitempty" yaml:"port,omitempty"`
	Encoding      string        `mapstructure:"encoding,omitempty" json:"encoding,omitempty" yaml:"encoding,omitempty"`
	Insecure      bool          `mapstructure:"insecure,omitempty" json:"insecure,omitempty" yaml:"insecure,omitempty"`
	TLSCa         string        `mapstructure:"tls-ca,omitempty" json:"tls-ca,omitempty" yaml:"tls-ca,omitempty"`
	TLSCert       string        `mapstructure:"tls-cert,omitempty" json:"tls-cert,omitempty" yaml:"tls-cert,omitempty"`
	TLSKey        string        `mapstructure:"tls-key,omitempty" json:"tls-key,omitempty" yaml:"tls-key,omitempty"`
	TLSMinVersion string        `mapstructure:"tls-min-version,omitempty" json:"tls-min-version,omitempty" yaml:"tls-min-version,omitempty"`
	TLSMaxVersion string        `mapstructure:"tls-max-version,omitempty" json:"tls-max-version,omitempty" yaml:"tls-max-version,omitempty"`
	TLSVersion    string        `mapstructure:"tls-version,omitempty" json:"tls-version,omitempty" yaml:"tls-version,omitempty"`
	TLSServerName string        `mapstructure:"tls-server-name,omitempty" json:"tls-server-name,omitempty" yaml:"tls-server-name,omitempty"`
	LogTLSSecret  bool          `mapstructure:"log-tls-secret,omitempty" json:"log-tls-secret,omitempty" yaml:"log-tls-secret,omitempty"`
	Timeout       time.Duration `mapstructure:"timeout,omitempty" json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Debug         bool          `mapstructure:"debug,omitempty" json:"debug,omitempty" yaml:"debug,omitempty"`
	EnablePprof   bool          `mapstructure:"enable-pprof,omitempty" json:"enable-pprof,omitempty" yaml:"enable-pprof,omitempty"`
	PprofAddr     string        `mapstructure:"pprof-addr,omitempty" json:"pprof-addr,omitempty" yaml:"pprof-addr,omitempty"`
	SkipVerify    bool          `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty" yaml:"skip-verify,omitempty"`
	NoPrefix      bool          `mapstructure:"no-prefix,omitempty" json:"no-prefix,omitempty" yaml:"no-prefix,omitempty"`
	ProxyFromEnv  bool          `mapstructure:"proxy-from-env,omitempty" json:"proxy-from-env,omitempty" yaml:"proxy-from-env,omitempty"`
	Format        string        `mapstructure:"format,omitempty" json:"format,omitempty" yaml:"format,omitempty"`
	LogFile       string        `mapstructure:"log-file,omitempty" json:"log-file,omitempty" yaml:"log-file,omitempty"`
	Log           bool          `mapstructure:"log,omitempty" json:"log,omitempty" yaml:"log,omitempty"`
	LogMaxSize    int           `mapstructure:"log-max-size,omitempty" json:"log-max-size,omitempty" yaml:"log-max-size,omitempty"`
	LogMaxBackups int           `mapstructure:"log-max-backups,omitempty" json:"log-max-backups,omitempty" yaml:"log-max-backups,omitempty"`
	LogCompress   bool          `mapstructure:"log-compress,omitempty" json:"log-compress,omitempty" yaml:"log-compress,omitempty"`
	MaxMsgSize    int           `mapstructure:"max-msg-size,omitempty" json:"max-msg-size,omitempty" yaml:"max-msg-size,omitempty"`
	//PrometheusAddress string        `mapstructure:"prometheus-address,omitempty" json:"prometheus-address,omitempty" yaml:"prometheus-address,omitempty"`
	PrintRequest     bool          `mapstructure:"print-request,omitempty" json:"print-request,omitempty" yaml:"print-request,omitempty"`
	Retry            time.Duration `mapstructure:"retry,omitempty" json:"retry,omitempty" yaml:"retry,omitempty"`
	TargetBufferSize uint          `mapstructure:"target-buffer-size,omitempty" json:"target-buffer-size,omitempty" yaml:"target-buffer-size,omitempty"`
	ClusterName      string        `mapstructure:"cluster-name,omitempty" json:"cluster-name,omitempty" yaml:"cluster-name,omitempty"`
	InstanceName     string        `mapstructure:"instance-name,omitempty" json:"instance-name,omitempty" yaml:"instance-name,omitempty"`
	API              string        `mapstructure:"api,omitempty" json:"api,omitempty" yaml:"api,omitempty"`
	ProtoFile        []string      `mapstructure:"proto-file,omitempty" json:"proto-file,omitempty" yaml:"proto-file,omitempty"`
	ProtoDir         []string      `mapstructure:"proto-dir,omitempty" json:"proto-dir,omitempty" yaml:"proto-dir,omitempty"`
	TargetsFile      string        `mapstructure:"targets-file,omitempty" json:"targets-file,omitempty" yaml:"targets-file,omitempty"`
	Gzip             bool          `mapstructure:"gzip,omitempty" json:"gzip,omitempty" yaml:"gzip,omitempty"`
	File             []string      `mapstructure:"file,omitempty" json:"file,omitempty" yaml:"file,omitempty"`
	Dir              []string      `mapstructure:"dir,omitempty" json:"dir,omitempty" yaml:"dir,omitempty"`
	Exclude          []string      `mapstructure:"exclude,omitempty" json:"exclude,omitempty" yaml:"exclude,omitempty"`
	Token            string        `mapstructure:"token,omitempty" json:"token,omitempty" yaml:"token,omitempty"`
	UseTunnelServer  bool          `mapstructure:"use-tunnel-server,omitempty" json:"use-tunnel-server,omitempty" yaml:"use-tunnel-server,omitempty"`
	AuthScheme       string        `mapstructure:"auth-scheme,omitempty" json:"auth-scheme,omitempty" yaml:"auth-scheme,omitempty"`
	CalculateLatency bool          `mapstructure:"calculate-latency,omitempty" json:"calculate-latency,omitempty" yaml:"calculate-latency,omitempty"`

	Metadata             map[string]string `mapstructure:"metadata,omitempty" json:"metadata,omitempty" yaml:"metadata,omitempty"`
	PluginProcessorsPath string            `mapstructure:"plugin-processors-path,omitempty" yaml:"plugin-processors-path,omitempty" json:"plugin-processors-path,omitempty"`
}

type LocalFlags struct {
	// Capabilities
	CapabilitiesVersion bool `mapstructure:"capabilities-version,omitempty" json:"capabilities-version,omitempty" yaml:"capabilities-version,omitempty"`
	// Get
	GetPath       []string `mapstructure:"get-path,omitempty" json:"get-path,omitempty" yaml:"get-path,omitempty"`
	GetPrefix     string   `mapstructure:"get-prefix,omitempty" json:"get-prefix,omitempty" yaml:"get-prefix,omitempty"`
	GetModel      []string `mapstructure:"get-model,omitempty" json:"get-model,omitempty" yaml:"get-model,omitempty"`
	GetType       string   `mapstructure:"get-type,omitempty" json:"get-type,omitempty" yaml:"get-type,omitempty"`
	GetTarget     string   `mapstructure:"get-target,omitempty" json:"get-target,omitempty" yaml:"get-target,omitempty"`
	GetValuesOnly bool     `mapstructure:"get-values-only,omitempty" json:"get-values-only,omitempty" yaml:"get-values-only,omitempty"`
	GetProcessor  []string `mapstructure:"get-processor,omitempty" json:"get-processor,omitempty" yaml:"get-processor,omitempty"`
	GetDepth      uint32   `mapstructure:"get-depth,omitempty" yaml:"get-depth,omitempty" json:"get-depth,omitempty"`
	GetDryRun     bool     `mapstructure:"get-dry-run,omitempty" json:"get-dry-run,omitempty" yaml:"get-dry-run,omitempty"`
	// Set
	SetPrefix                 string        `mapstructure:"set-prefix,omitempty" json:"set-prefix,omitempty" yaml:"set-prefix,omitempty"`
	SetDelete                 []string      `mapstructure:"set-delete,omitempty" json:"set-delete,omitempty" yaml:"set-delete,omitempty"`
	SetReplace                []string      `mapstructure:"set-replace,omitempty" json:"set-replace,omitempty" yaml:"set-replace,omitempty"`
	SetUnionReplace           []string      `mapstructure:"set-union-replace,omitempty" json:"set-union-replace,omitempty" yaml:"set-union-replace,omitempty"`
	SetUpdate                 []string      `mapstructure:"set-update,omitempty" json:"set-update,omitempty" yaml:"set-update,omitempty"`
	SetReplacePath            []string      `mapstructure:"set-replace-path,omitempty" json:"set-replace-path,omitempty" yaml:"set-replace-path,omitempty"`
	SetUpdatePath             []string      `mapstructure:"set-update-path,omitempty" json:"set-update-path,omitempty" yaml:"set-update-path,omitempty"`
	SetReplaceFile            []string      `mapstructure:"set-replace-file,omitempty" json:"set-replace-file,omitempty" yaml:"set-replace-file,omitempty"`
	SetUpdateFile             []string      `mapstructure:"set-update-file,omitempty" json:"set-update-file,omitempty" yaml:"set-update-file,omitempty"`
	SetReplaceValue           []string      `mapstructure:"set-replace-value,omitempty" json:"set-replace-value,omitempty" yaml:"set-replace-value,omitempty"`
	SetUpdateValue            []string      `mapstructure:"set-update-value,omitempty" json:"set-update-value,omitempty" yaml:"set-update-value,omitempty"`
	SetUnionReplacePath       []string      `mapstructure:"set-union-replace-path,omitempty" yaml:"set-union-replace-path,omitempty" json:"set-union-replace-path,omitempty"`
	SetUnionReplaceValue      []string      `mapstructure:"set-union-replace-value,omitempty" yaml:"set-union-replace-value,omitempty" json:"set-union-replace-value,omitempty"`
	SetUnionReplaceFile       []string      `mapstructure:"set-union-replace-file,omitempty" yaml:"set-union-replace-file,omitempty" json:"set-union-replace-file,omitempty"`
	SetDelimiter              string        `mapstructure:"set-delimiter,omitempty" json:"set-delimiter,omitempty" yaml:"set-delimiter,omitempty"`
	SetTarget                 string        `mapstructure:"set-target,omitempty" json:"set-target,omitempty" yaml:"set-target,omitempty"`
	SetRequestFile            []string      `mapstructure:"set-request-file,omitempty" json:"set-request-file,omitempty" yaml:"set-request-file,omitempty"`
	SetRequestVars            string        `mapstructure:"set-request-vars,omitempty" json:"set-request-vars,omitempty" yaml:"set-request-vars,omitempty"`
	SetRequestProtoFile       []string      `mapstructure:"set-proto-request-file,omitempty" yaml:"set-proto-request-file,omitempty" json:"set-proto-request-file,omitempty"`
	SetDryRun                 bool          `mapstructure:"set-dry-run,omitempty" json:"set-dry-run,omitempty" yaml:"set-dry-run,omitempty"`
	SetNoTrim                 bool          `mapstructure:"set-no-trim,omitempty" json:"set-no-trim,omitempty" yaml:"set-no-trim,omitempty"`
	SetReplaceCli             []string      `mapstructure:"set-replace-cli,omitempty" yaml:"set-replace-cli,omitempty" json:"set-replace-cli,omitempty"`
	SetReplaceCliFile         string        `mapstructure:"set-replace-cli-file,omitempty" yaml:"set-replace-cli-file,omitempty" json:"set-replace-cli-file,omitempty"`
	SetUpdateCli              []string      `mapstructure:"set-update-cli,omitempty" yaml:"set-update-cli,omitempty" json:"set-update-cli,omitempty"`
	SetUpdateCliFile          string        `mapstructure:"set-update-cli-file,omitempty" yaml:"set-update-cli-file,omitempty" json:"set-update-cli-file,omitempty"`
	SetCommitId               string        `mapstructure:"set-commit-id,omitempty" yaml:"set-commit-id,omitempty" json:"set-commit-id,omitempty"`
	SetCommitRequest          bool          `mapstructure:"set-commit-request,omitempty" yaml:"set-commit-request,omitempty" json:"set-commit-request,omitempty"`
	SetCommitRollbackDuration time.Duration `mapstructure:"set-commit-rollback-duration,omitempty" yaml:"set-commit-rollback-duration,omitempty" json:"set-commit-rollback-duration,omitempty"`
	SetCommitCancel           bool          `mapstructure:"set-commit-cancel,omitempty" yaml:"set-commit-cancel,omitempty" json:"set-commit-cancel,omitempty"`
	SetCommitConfirm          bool          `mapstructure:"set-commit-confirm,omitempty" yaml:"set-commit-confirm,omitempty" json:"set-commit-confirm,omitempty"`
	// Sub
	SubscribePrefix            string        `mapstructure:"subscribe-prefix,omitempty" json:"subscribe-prefix,omitempty" yaml:"subscribe-prefix,omitempty"`
	SubscribePath              []string      `mapstructure:"subscribe-path,omitempty" json:"subscribe-path,omitempty" yaml:"subscribe-path,omitempty"`
	SubscribeQos               uint32        `mapstructure:"subscribe-qos,omitempty" json:"subscribe-qos,omitempty" yaml:"subscribe-qos,omitempty"`
	SubscribeUpdatesOnly       bool          `mapstructure:"subscribe-updates-only,omitempty" json:"subscribe-updates-only,omitempty" yaml:"subscribe-updates-only,omitempty"`
	SubscribeMode              string        `mapstructure:"subscribe-mode,omitempty" json:"subscribe-mode,omitempty" yaml:"subscribe-mode,omitempty"`
	SubscribeStreamMode        string        `mapstructure:"subscribe-stream_mode,omitempty" json:"subscribe-stream-mode,omitempty" yaml:"subscribe-stream-mode,omitempty"`
	SubscribeSampleInterval    time.Duration `mapstructure:"subscribe-sample-interval,omitempty" json:"subscribe-sample-interval,omitempty" yaml:"subscribe-sample-interval,omitempty"`
	SubscribeSuppressRedundant bool          `mapstructure:"subscribe-suppress-redundant,omitempty" json:"subscribe-suppress-redundant,omitempty" yaml:"subscribe-suppress-redundant,omitempty"`
	SubscribeHeartbeatInterval time.Duration `mapstructure:"subscribe-heartbeat-interval,omitempty" json:"subscribe-heartbeat-interval,omitempty" yaml:"subscribe-heartbeat-interval,omitempty"`
	SubscribeModel             []string      `mapstructure:"subscribe-model,omitempty" json:"subscribe-model,omitempty" yaml:"subscribe-model,omitempty"`
	SubscribeQuiet             bool          `mapstructure:"subscribe-quiet,omitempty" json:"subscribe-quiet,omitempty" yaml:"subscribe-quiet,omitempty"`
	SubscribeTarget            string        `mapstructure:"subscribe-target,omitempty" json:"subscribe-target,omitempty" yaml:"subscribe-target,omitempty"`
	SubscribeSetTarget         bool          `mapstructure:"subscribe-set-target,omitempty" json:"subscribe-set-target,omitempty" yaml:"subscribe-set-target,omitempty"`
	SubscribeName              []string      `mapstructure:"subscribe-name,omitempty" json:"subscribe-name,omitempty" yaml:"subscribe-name,omitempty"`
	SubscribeOutput            []string      `mapstructure:"subscribe-output,omitempty" json:"subscribe-output,omitempty" yaml:"subscribe-output,omitempty"`
	SubscribeWatchConfig       bool          `mapstructure:"subscribe-watch-config,omitempty" json:"subscribe-watch-config,omitempty" yaml:"subscribe-watch-config,omitempty"`
	SubscribeBackoff           time.Duration `mapstructure:"subscribe-backoff,omitempty" json:"subscribe-backoff,omitempty" yaml:"subscribe-backoff,omitempty"`
	SubscribeLockRetry         time.Duration `mapstructure:"subscribe-lock-retry,omitempty" json:"subscribe-lock-retry,omitempty" yaml:"subscribe-lock-retry,omitempty"`
	SubscribeHistorySnapshot   string        `mapstructure:"subscribe-history-snapshot,omitempty" json:"subscribe-history-snapshot,omitempty" yaml:"subscribe-history-snapshot,omitempty"`
	SubscribeHistoryStart      string        `mapstructure:"subscribe-history-start,omitempty" json:"subscribe-history-start,omitempty" yaml:"subscribe-history-start,omitempty"`
	SubscribeHistoryEnd        string        `mapstructure:"subscribe-history-end,omitempty" json:"subscribe-history-end,omitempty" yaml:"subscribe-history-end,omitempty"`
	SubscribeDepth             uint32        `mapstructure:"subscribe-depth,omitempty" yaml:"subscribe-depth,omitempty" json:"subscribe-depth,omitempty"`
	// Path
	PathPathType   string `mapstructure:"path-path-type,omitempty" json:"path-path-type,omitempty" yaml:"path-path-type,omitempty"`
	PathWithDescr  bool   `mapstructure:"path-descr,omitempty" json:"path-descr,omitempty" yaml:"path-descr,omitempty"`
	PathWithPrefix bool   `mapstructure:"path-with-prefix,omitempty" json:"path-with-prefix,omitempty" yaml:"path-with-prefix,omitempty"`
	PathWithTypes  bool   `mapstructure:"path-types,omitempty" json:"path-types,omitempty" yaml:"path-types,omitempty"`
	PathSearch     bool   `mapstructure:"path-search,omitempty" json:"path-search,omitempty" yaml:"path-search,omitempty"`
	PathState      bool   `mapstructure:"path-state,omitempty" json:"path-state,omitempty" yaml:"path-state,omitempty"`
	PathConfig     bool   `mapstructure:"path-config,omitempty" json:"path-config,omitempty" yaml:"path-config,omitempty"`
	// Prompt
	PromptFile                  []string `mapstructure:"prompt-file,omitempty" json:"prompt-file,omitempty" yaml:"prompt-file,omitempty"`
	PromptExclude               []string `mapstructure:"prompt-exclude,omitempty" json:"prompt-exclude,omitempty" yaml:"prompt-exclude,omitempty"`
	PromptDir                   []string `mapstructure:"prompt-dir,omitempty" json:"prompt-dir,omitempty" yaml:"prompt-dir,omitempty"`
	PromptMaxSuggestions        uint16   `mapstructure:"prompt-max-suggestions,omitempty" json:"prompt-max-suggestions,omitempty" yaml:"prompt-max-suggestions,omitempty"`
	PromptPrefixColor           string   `mapstructure:"prompt-prefix-color,omitempty" json:"prompt-prefix-color,omitempty" yaml:"prompt-prefix-color,omitempty"`
	PromptSuggestionsBGColor    string   `mapstructure:"prompt-suggestions-bg-color,omitempty" json:"prompt-suggestions-bg-color,omitempty" yaml:"prompt-suggestions-bg-color,omitempty"`
	PromptDescriptionBGColor    string   `mapstructure:"prompt-description-bg-color,omitempty" json:"prompt-description-bg-color,omitempty" yaml:"prompt-description-bg-color,omitempty"`
	PromptSuggestAllFlags       bool     `mapstructure:"prompt-suggest-all-flags,omitempty" json:"prompt-suggest-all-flags,omitempty" yaml:"prompt-suggest-all-flags,omitempty"`
	PromptDescriptionWithPrefix bool     `mapstructure:"prompt-description-with-prefix,omitempty" json:"prompt-description-with-prefix,omitempty" yaml:"prompt-description-with-prefix,omitempty"`
	PromptDescriptionWithTypes  bool     `mapstructure:"prompt-description-with-types,omitempty" json:"prompt-description-with-types,omitempty" yaml:"prompt-description-with-types,omitempty"`
	PromptSuggestWithOrigin     bool     `mapstructure:"prompt-suggest-with-origin,omitempty" json:"prompt-suggest-with-origin,omitempty" yaml:"prompt-suggest-with-origin,omitempty"`
	// Listen
	ListenMaxConcurrentStreams uint32 `mapstructure:"listen-max-concurrent-streams,omitempty" json:"listen-max-concurrent-streams,omitempty" yaml:"listen-max-concurrent-streams,omitempty"`
	ListenPrometheusAddress    string `mapstructure:"listen-prometheus-address,omitempty" json:"listen-prometheus-address,omitempty" yaml:"listen-prometheus-address,omitempty"`
	// VersionUpgrade
	UpgradeUsePkg bool `mapstructure:"upgrade-use-pkg" json:"upgrade-use-pkg,omitempty" yaml:"upgrade-use-pkg,omitempty"`
	// GetSet
	GetSetPrefix    string   `mapstructure:"getset-prefix,omitempty" json:"getset-prefix,omitempty" yaml:"getset-prefix,omitempty"`
	GetSetGet       string   `mapstructure:"getset-get,omitempty" json:"getset-get,omitempty" yaml:"getset-get,omitempty"`
	GetSetModel     []string `mapstructure:"get-set-model,omitempty" yaml:"get-set-model,omitempty" json:"get-set-model,omitempty"`
	GetSetTarget    string   `mapstructure:"getset-target,omitempty" json:"getset-target,omitempty" yaml:"getset-target,omitempty"`
	GetSetType      string   `mapstructure:"getset-type,omitempty" json:"getset-type,omitempty" yaml:"getset-type,omitempty"`
	GetSetCondition string   `mapstructure:"getset-condition,omitempty" json:"getset-condition,omitempty" yaml:"getset-condition,omitempty"`
	GetSetUpdate    string   `mapstructure:"getset-update,omitempty" json:"getset-update,omitempty" yaml:"getset-update,omitempty"`
	GetSetReplace   string   `mapstructure:"getset-replace,omitempty" json:"getset-replace,omitempty" yaml:"getset-replace,omitempty"`
	GetSetDelete    string   `mapstructure:"getset-delete,omitempty" json:"getset-delete,omitempty" yaml:"getset-delete,omitempty"`
	GetSetValue     string   `mapstructure:"getset-value,omitempty" json:"getset-value,omitempty" yaml:"getset-value,omitempty"`
	// Generate
	GenerateOutput     string `mapstructure:"generate-output,omitempty" json:"generate-output,omitempty" yaml:"generate-output,omitempty"`
	GenerateJSON       bool   `mapstructure:"generate-json,omitempty" json:"generate-json,omitempty" yaml:"generate-json,omitempty"`
	GenerateConfigOnly bool   `mapstructure:"generate-config-only,omitempty" json:"generate-config-only,omitempty" yaml:"generate-config-only,omitempty"`
	GeneratePath       string `mapstructure:"generate-path,omitempty" json:"generate-path,omitempty" yaml:"generate-path,omitempty"`
	GenerateCamelCase  bool   `mapstructure:"generate-camel-case,omitempty" json:"generate-camel-case,omitempty" yaml:"generate-camel-case,omitempty"`
	GenerateSnakeCase  bool   `mapstructure:"generate-snake-case,omitempty" json:"generate-snake-case,omitempty" yaml:"generate-snake-case,omitempty"`
	// Generate Set Request
	GenerateSetRequestUpdatePath  []string `mapstructure:"generate-update-path,omitempty" json:"generate-update-path,omitempty" yaml:"generate-update-path,omitempty"`
	GenerateSetRequestReplacePath []string `mapstructure:"generate-replace-path,omitempty" json:"generate-replace-path,omitempty" yaml:"generate-replace-path,omitempty"`
	// Generate path
	GeneratePathWithDescr     bool   `mapstructure:"generate-descr,omitempty" json:"generate-descr,omitempty" yaml:"generate-descr,omitempty"`
	GeneratePathWithPrefix    bool   `mapstructure:"generate-with-prefix,omitempty" json:"generate-with-prefix,omitempty" yaml:"generate-with-prefix,omitempty"`
	GeneratePathWithTypes     bool   `mapstructure:"generate-types,omitempty" json:"generate-types,omitempty" yaml:"generate-types,omitempty"`
	GeneratePathSearch        bool   `mapstructure:"generate-search,omitempty" json:"generate-search,omitempty" yaml:"generate-search,omitempty"`
	GeneratePathPathType      string `mapstructure:"generate-path-path-type,omitempty" json:"generate-path-path-type,omitempty" yaml:"generate-path-path-type,omitempty"`
	GeneratePathState         bool   `mapstructure:"generate-path-state,omitempty" json:"generate-path-state,omitempty" yaml:"generate-path-state,omitempty"`
	GeneratePathConfig        bool   `mapstructure:"generate-path-config,omitempty" json:"generate-path-config,omitempty" yaml:"generate-path-config,omitempty"`
	GeneratePathWithNonLeaves bool   `mapstructure:"generate-path-with-non-leaves,omitempty" json:"generate-path-with-non-leaves,omitempty" yaml:"generate-path-with-non-leaves,omitempty"`
	//
	DiffPath                []string `mapstructure:"diff-path,omitempty" json:"diff-path,omitempty" yaml:"diff-path,omitempty"`
	DiffPrefix              string   `mapstructure:"diff-prefix,omitempty" json:"diff-prefix,omitempty" yaml:"diff-prefix,omitempty"`
	DiffModel               []string `mapstructure:"diff-model,omitempty" json:"diff-model,omitempty" yaml:"diff-model,omitempty"`
	DiffType                string   `mapstructure:"diff-type,omitempty" json:"diff-type,omitempty" yaml:"diff-type,omitempty"`
	DiffTarget              string   `mapstructure:"diff-target,omitempty" json:"diff-target,omitempty" yaml:"diff-target,omitempty"`
	DiffSub                 bool     `mapstructure:"diff-sub,omitempty" json:"diff-sub,omitempty" yaml:"diff-sub,omitempty"`
	DiffRef                 string   `mapstructure:"diff-ref,omitempty" json:"diff-ref,omitempty" yaml:"diff-ref,omitempty"`
	DiffCompare             []string `mapstructure:"diff-compare,omitempty" json:"diff-compare,omitempty" yaml:"diff-compare,omitempty"`
	DiffQos                 uint32   `mapstructure:"diff-qos,omitempty" json:"diff-qos,omitempty" yaml:"diff-qos,omitempty"`
	DiffSetRequestRef       string   `mapstructure:"diff-setrequest-ref,omitempty" json:"diff-setrequest-ref,omitempty" yaml:"diff-setrequest-ref,omitempty"`
	DiffSetRequestNew       string   `mapstructure:"diff-setrequest-new,omitempty" json:"diff-setrequest-new,omitempty" yaml:"diff-setrequest-new,omitempty"`
	DiffSetRequestFull      bool     `mapstructure:"diff-setrequest-full,omitempty" json:"diff-setrequest-full,omitempty" yaml:"diff-setrequest-full,omitempty"`
	DiffSetToNotifsSet      string   `mapstructure:"diff-set-to-notifs-set,omitempty" json:"diff-set-to-notifs-set,omitempty" yaml:"diff-set-to-notifs-set,omitempty"`
	DiffSetToNotifsResponse string   `mapstructure:"diff-set-to-notifs-response,omitempty" json:"diff-set-to-notifs-response,omitempty" yaml:"diff-set-to-notifs-response,omitempty"`
	DiffSetToNotifsFull     bool     `mapstructure:"diff-set-to-notifs-full,omitempty" json:"diff-set-to-notifs-full,omitempty" yaml:"diff-set-to-notifs-full,omitempty"`
	//
	TunnelServerSubscribe bool `mapstructure:"tunnel-server-subscribe,omitempty" yaml:"tunnel-server-subscribe,omitempty" json:"tunnel-server-subscribe,omitempty"`
	// Processor
	ProcessorInput          string   `mapstructure:"processor-input,omitempty" yaml:"processor-input,omitempty" json:"processor-input,omitempty"`
	ProcessorInputDelimiter string   `mapstructure:"processor-input-delimiter,omitempty" yaml:"processor-input-delimiter,omitempty" json:"processor-input-delimiter,omitempty"`
	ProcessorName           []string `mapstructure:"processor-name,omitempty" yaml:"processor-name,omitempty" json:"processor-name,omitempty"`
	ProcessorOutput         string   `mapstructure:"processor-output,omitempty" yaml:"processor-output,omitempty" json:"processor-output,omitempty"`
}

func New() *Config {
	return &Config{
		GlobalFlags{},
		LocalFlags{},
		viper.NewWithOptions(viper.KeyDelimiter("/")),
		make(map[string]*types.TargetConfig),
		make(map[string]*types.SubscriptionConfig),
		make(map[string]map[string]interface{}),
		make(map[string]map[string]interface{}),
		make(map[string]map[string]interface{}),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		log.New(io.Discard, configLogPrefix, utils.DefaultLoggingFlags),
		nil,
		make(map[string]interface{}),
	}
}

func (c *Config) Load(ctx context.Context) error {
	c.FileConfig.SetEnvPrefix(envPrefix)
	c.FileConfig.SetEnvKeyReplacer(strings.NewReplacer("/", "_", "-", "_"))
	c.FileConfig.AutomaticEnv()
	if c.GlobalFlags.CfgFile != "" {
		// configuration file path is explicitly set
		c.FileConfig.SetConfigFile(c.GlobalFlags.CfgFile)
		configBytes, err := gfile.ReadFile(ctx, c.FileConfig.ConfigFileUsed())
		if err != nil {
			return err
		}
		err = c.FileConfig.ReadConfig(bytes.NewBuffer(configBytes))
		if err != nil {
			return err
		}
	} else {
		// discover gnmic config file
		home, err := homedir.Dir()
		if err != nil {
			return err
		}
		c.FileConfig.AddConfigPath(".")
		c.FileConfig.AddConfigPath(home)
		c.FileConfig.AddConfigPath(xdg.ConfigHome)
		c.FileConfig.AddConfigPath(xdg.ConfigHome + "/gnmic")
		c.FileConfig.SetConfigName(configName)
		err = c.FileConfig.ReadInConfig()
		if err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return err
			}
		}
	}

	err := c.FileConfig.Unmarshal(c)
	if err != nil {
		return err
	}

	c.mergeEnvVars()
	return c.expandOSPathFlagValues()
}

func (c *Config) ToStore(s store.Store[any]) error {
	targets := make(map[string]any)
	subscriptions := make(map[string]any)
	processors := make(map[string]any)
	outputs := make(map[string]any)
	inputs := make(map[string]any)
	actions := make(map[string]any)
	_, err := c.GetTargets()
	if err != nil {
		if !errors.Is(err, ErrNoTargetsFound) {
			return err
		}
	}
	// targets
	for n, t := range c.Targets {
		targets[n] = t
	}
	// subscriptions
	for n, s := range c.Subscriptions {
		subscriptions[n] = s
	}
	// processors
	for n, p := range c.Processors {
		processors[n] = p
	}
	// outputs
	for n, o := range c.Outputs {
		outputs[n] = o
	}
	// inputs
	for n, i := range c.Inputs {
		inputs[n] = i
	}
	// actions
	for n, a := range c.Actions {
		actions[n] = a
	}
	// set all
	err = s.SetAll("targets", targets)
	if err != nil {
		return err
	}
	err = s.SetAll("subscriptions", subscriptions)
	if err != nil {
		return err
	}
	// actions
	err = s.SetAll("actions", actions)
	if err != nil {
		return err
	}
	err = s.SetAll("processors", processors)
	if err != nil {
		return err
	}
	err = s.SetAll("outputs", outputs)
	if err != nil {
		return err
	}
	err = s.SetAll("inputs", inputs)
	if err != nil {
		return err
	}
	//
	_, err = s.Set("global-flags", "global-flags", c.GlobalFlags)
	if err != nil {
		return err
	}
	// clustering
	_, err = s.Set("clustering", "clustering", c.Clustering)
	if err != nil {
		return err
	}
	// gnmi server
	_, err = s.Set("gnmi-server", "gnmi-server", c.GnmiServer)
	if err != nil {
		return err
	}
	// api server
	_, err = s.Set("api-server", "api-server", c.APIServer)
	if err != nil {
		return err
	}
	// loader
	_, err = s.Set("loader", "loader", c.Loader)
	if err != nil {
		return err
	}
	// tunnel server
	_, err = s.Set("tunnel-server", "tunnel-server", c.TunnelServer)
	if err != nil {
		return err
	}
	return nil
}

func (c *Config) SetLogger() (io.Writer, int, error) {
	var f io.Writer = io.Discard
	var loggingFlags = c.logger.Flags()
	var err error

	if c.LogFile != "" {
		if c.LogMaxSize > 0 {
			f = &lumberjack.Logger{
				Filename:   c.LogFile,
				MaxSize:    c.LogMaxSize,
				MaxBackups: c.LogMaxBackups,
				Compress:   c.LogCompress,
			}
		} else {
			f, err = os.OpenFile(c.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				return nil, 0, err
			}
		}
	} else {
		if c.Debug {
			c.Log = true
		}
		if c.Log {
			f = os.Stderr
		}
	}
	if c.Debug {
		loggingFlags |= log.Llongfile
	}
	c.logger.SetOutput(f)
	c.logger.SetFlags(loggingFlags)
	return f, loggingFlags, nil
}

func (c *Config) SetPersistentFlagsFromFile(cmd *cobra.Command) {
	// set debug and log values from file before other persistent flags
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "debug" || f.Name == "log" {
			if !f.Changed && c.FileConfig.IsSet(f.Name) {
				c.setFlagValue(cmd, f.Name, c.FileConfig.Get(f.Name))
			}
		}
	})
	//
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "debug" || f.Name == "log" {
			return
		}
		if c.Debug {
			c.logger.Printf("cmd=%s, flagName=%s, changed=%v, isSetInFile=%v",
				cmd.Name(), f.Name, f.Changed, c.FileConfig.IsSet(f.Name))
		}
		if !f.Changed && c.FileConfig.IsSet(f.Name) {
			c.setFlagValue(cmd, f.Name, c.FileConfig.Get(f.Name))
		}
	})
}

func (c *Config) SetLocalFlagsFromFile(cmd *cobra.Command) {
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		flagName := fmt.Sprintf("%s-%s", cmd.Name(), f.Name)
		if c.Debug {
			c.logger.Printf("cmd=%s, flagName=%s, changed=%v, isSetInFile=%v",
				cmd.Name(), f.Name, f.Changed, c.FileConfig.IsSet(flagName))
		}
		if !f.Changed && c.FileConfig.IsSet(flagName) {
			c.setFlagValue(cmd, f.Name, c.FileConfig.Get(flagName))
		}
	})
}

func (c *Config) setFlagValue(cmd *cobra.Command, fName string, val interface{}) {
	switch val := val.(type) {
	case []interface{}:
		if c.Debug {
			c.logger.Printf("cmd=%s, flagName=%s, valueType=%T, length=%d, value=%#v",
				cmd.Name(), fName, val, len(val), val)
		}
		nVal := make([]string, 0, len(val))
		for _, v := range val {
			nVal = append(nVal, fmt.Sprintf("%v", v))
		}
		cmd.Flags().Set(fName, strings.Join(nVal, ","))
	default:
		if c.Debug {
			c.logger.Printf("cmd=%s, flagName=%s, valueType=%T, value=%#v",
				cmd.Name(), fName, val, val)
		}
		cmd.Flags().Set(fName, fmt.Sprintf("%v", val))
	}
}

func flagIsSet(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	var isSet bool
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name == name && f.Changed {
			isSet = true
			return
		}
	})
	return isSet
}

func (c *Config) CreateGetRequest(tc *types.TargetConfig) (*gnmi.GetRequest, error) {
	if c == nil {
		return nil, fmt.Errorf("%w", ErrInvalidConfig)
	}
	gnmiOpts := make([]api.GNMIOption, 0, 4+len(c.LocalFlags.GetPath))
	enc := c.Encoding
	if tc.Encoding != nil {
		enc = *tc.Encoding
	}
	gnmiOpts = append(gnmiOpts,
		api.Encoding(enc),
		api.DataType(c.LocalFlags.GetType),
		api.Prefix(c.LocalFlags.GetPrefix),
		api.Target(c.LocalFlags.GetTarget),
	)
	for _, p := range c.LocalFlags.GetPath {
		gnmiOpts = append(gnmiOpts, api.Path(strings.TrimSpace(p)))
	}
	if c.LocalFlags.GetDepth > 0 {
		gnmiOpts = append(gnmiOpts, api.Extension_Depth(c.LocalFlags.GetDepth))
	}
	return api.NewGetRequest(gnmiOpts...)
}

func (c *Config) CreateGASGetRequest() (*gnmi.GetRequest, error) {
	if c == nil {
		return nil, fmt.Errorf("%w", ErrInvalidConfig)
	}
	return api.NewGetRequest(
		api.Encoding(c.Encoding),
		api.DataType(c.LocalFlags.GetSetType),
		api.Prefix(c.LocalFlags.GetSetPrefix),
		api.Target(c.LocalFlags.GetSetTarget),
		api.Path(strings.TrimSpace(c.LocalFlags.GetSetGet)))
}

func (c *Config) CreateGASSetRequest(input interface{}) (*gnmi.SetRequest, error) {
	gnmiOpts := make([]api.GNMIOption, 0, 3)
	gnmiOpts = append(gnmiOpts, api.Prefix(c.LocalFlags.GetSetPrefix))
	gnmiOpts = append(gnmiOpts, api.Target(c.LocalFlags.GetSetTarget))

	delPath, err := c.execPathTemplate(c.LocalFlags.GetSetDelete, input)
	if err != nil {
		return nil, err
	}
	if delPath != "" {
		gnmiOpts = append(gnmiOpts, api.Delete(delPath))
	}
	//
	updatePath, err := c.execPathTemplate(c.LocalFlags.GetSetUpdate, input)
	if err != nil {
		return nil, err
	}
	replacePath, err := c.execPathTemplate(c.LocalFlags.GetSetReplace, input)
	if err != nil {
		return nil, err
	}
	val, err := c.execValueTemplate(c.LocalFlags.GetSetValue, input)
	if err != nil {
		return nil, err
	}
	if updatePath != "" {
		gnmiOpts = append(gnmiOpts,
			api.Update(
				api.Path(updatePath),
				api.Value(val, c.Encoding),
			))
	} else if replacePath != "" {
		gnmiOpts = append(gnmiOpts,
			api.Replace(
				api.Path(replacePath),
				api.Value(val, c.Encoding),
			))
	}

	return api.NewSetRequest(gnmiOpts...)
}

func (c *Config) execPathTemplate(tplString string, input interface{}) (string, error) {
	if tplString == "" {
		return "", nil
	}
	tplString = os.ExpandEnv(tplString)
	q, err := gojq.Parse(tplString)
	if err != nil {
		return "", err
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return "", err
	}
	iter := code.Run(input)
	var res interface{}
	var ok bool

	res, ok = iter.Next()
	if !ok {
		if c.Debug {
			c.logger.Printf("jq input: %+v", input)
			c.logger.Printf("jq result: %+v", res)
		}
		return "", fmt.Errorf("unexpected jq result type: %T", res)
	}
	switch v := res.(type) {
	case error:
		return "", v
	case string:
		c.logger.Printf("path jq expression result: %s", v)
		return v, nil
	default:
		if c.Debug {
			c.logger.Printf("jq input: %+v", input)
			c.logger.Printf("jq result: %+v", v)
		}
		return "", fmt.Errorf("unexpected jq result type: %T", v)
	}
}

func (c *Config) execValueTemplate(tplString string, input interface{}) (string, error) {
	if tplString == "" {
		return "", nil
	}
	tplString = os.ExpandEnv(tplString)
	q, err := gojq.Parse(tplString)
	if err != nil {
		return "", err
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return "", err
	}
	iter := code.Run(input)
	var res interface{}
	var ok bool

	res, ok = iter.Next()
	if !ok {
		if c.Debug {
			c.logger.Printf("jq input: %+v", input)
			c.logger.Printf("jq result: %+v", res)
		}
		return "", fmt.Errorf("unexpected jq result type: %T", res)
	}
	switch v := res.(type) {
	case error:
		return "", v
	case string:
		c.logger.Printf("path jq expression result: %s", v)
		return trimQuotes(v), nil
	default:
		if c.Debug {
			c.logger.Printf("jq input: %+v", input)
			c.logger.Printf("jq result: %+v", v)
		}
		return "", fmt.Errorf("unexpected jq result type: %T", v)
	}
}

func (c *Config) CreateSetRequest(targetName string) ([]*gnmi.SetRequest, error) {
	if len(c.SetRequestProtoFile) > 0 {
		return c.CreateSetRequestFromProtoFile()
	}
	if len(c.SetRequestFile) > 0 {
		return c.CreateSetRequestFromFile(targetName)
	}
	if c.Debug {
		c.logger.Printf("Set input delete: %+v", &c.LocalFlags.SetDelete)
		c.logger.Printf("Set input update: %+v", &c.LocalFlags.SetUpdate)
		c.logger.Printf("Set input update path(s): %+v", &c.LocalFlags.SetUpdatePath)
		c.logger.Printf("Set input update value(s): %+v", &c.LocalFlags.SetUpdateValue)
		c.logger.Printf("Set input update file(s): %+v", &c.LocalFlags.SetUpdateFile)
		c.logger.Printf("Set input replace: %+v", &c.LocalFlags.SetReplace)
		c.logger.Printf("Set input replace path(s): %+v", &c.LocalFlags.SetReplacePath)
		c.logger.Printf("Set input replace value(s): %+v", &c.LocalFlags.SetReplaceValue)
		c.logger.Printf("Set input replace file(s): %+v", &c.LocalFlags.SetReplaceFile)
		c.logger.Printf("Set input union replace path(s): %+v", &c.LocalFlags.SetUnionReplacePath)
		c.logger.Printf("Set input union replace value(s): %+v", &c.LocalFlags.SetUnionReplaceValue)
		c.logger.Printf("Set input union replace file(s): %+v", &c.LocalFlags.SetUnionReplaceFile)
	}

	gnmiOpts := make([]api.GNMIOption, 0, 2+ // prefix+target
		len(c.LocalFlags.SetDelete)+len(c.LocalFlags.SetUpdate)+len(c.LocalFlags.SetReplace)+len(c.LocalFlags.SetUnionReplace)+
		len(c.LocalFlags.SetUpdatePath)+len(c.LocalFlags.SetReplacePath)+len(c.LocalFlags.SetUnionReplacePath)+
		1+1+ // updateCli + replaceCli
		1+1, // updateCliFile + replaceCliFile
	)
	gnmiOpts = append(gnmiOpts,
		api.Prefix(c.LocalFlags.SetPrefix),
		api.Target(c.LocalFlags.SetTarget),
	)
	for _, p := range c.LocalFlags.SetDelete {
		gnmiOpts = append(gnmiOpts, api.Delete(strings.TrimSpace(p)))
	}

	for _, u := range c.LocalFlags.SetUpdate {
		singleUpdate := strings.SplitN(u, c.LocalFlags.SetDelimiter, 3)
		if len(singleUpdate) < 3 {
			return nil, fmt.Errorf("invalid inline update format: %s", c.LocalFlags.SetUpdate)
		}
		gnmiOpts = append(gnmiOpts,
			api.Update(
				api.Path(strings.TrimSpace(singleUpdate[0])),
				api.Value(singleUpdate[2], singleUpdate[1]),
			),
		)
	}

	for _, r := range c.LocalFlags.SetReplace {
		singleReplace := strings.SplitN(r, c.LocalFlags.SetDelimiter, 3)
		if len(singleReplace) < 3 {
			return nil, fmt.Errorf("invalid inline replace format: %s", c.LocalFlags.SetReplace)
		}
		gnmiOpts = append(gnmiOpts,
			api.Replace(
				api.Path(strings.TrimSpace(singleReplace[0])),
				api.Value(singleReplace[2], singleReplace[1]),
			),
		)
	}

	for _, r := range c.LocalFlags.SetUnionReplace {
		singleUnionReplace := strings.SplitN(r, c.LocalFlags.SetDelimiter, 3)
		if len(singleUnionReplace) < 3 {
			return nil, fmt.Errorf("invalid inline union-replace format: %s'", c.LocalFlags.SetReplace)
		}
		gnmiOpts = append(gnmiOpts,
			api.UnionReplace(
				api.Path(strings.TrimSpace(singleUnionReplace[0])),
				api.Value(singleUnionReplace[2], singleUnionReplace[1]),
			),
		)
	}

	useUpdateFiles := len(c.LocalFlags.SetUpdateFile) > 0 && len(c.LocalFlags.SetUpdateValue) == 0
	useReplaceFiles := len(c.LocalFlags.SetReplaceFile) > 0 && len(c.LocalFlags.SetReplaceValue) == 0
	useUnionReplaceFiles := len(c.LocalFlags.SetUnionReplaceFile) > 0 && len(c.LocalFlags.SetUnionReplaceValue) == 0

	for i, p := range c.LocalFlags.SetUpdatePath {
		var updOpt api.GNMIOption
		if useUpdateFiles {
			updateData, err := readFile(c.LocalFlags.SetUpdateFile[i])
			if err != nil {
				c.logger.Printf("error reading data from file '%s': %v", c.LocalFlags.SetUpdateFile[i], err)
				return nil, err
			}
			trim := ""
			if !c.LocalFlags.SetNoTrim {
				trim = trimChars
			}
			updOpt = api.Update(
				api.Path(strings.TrimSpace(p)),
				api.Value(string(bytes.Trim(updateData, trim)), c.Encoding),
			)

		} else {
			updOpt = api.Update(
				api.Path(strings.TrimSpace(p)),
				api.Value(c.LocalFlags.SetUpdateValue[i], c.Encoding),
			)
		}
		gnmiOpts = append(gnmiOpts, updOpt)
	}

	for i, p := range c.LocalFlags.SetReplacePath {
		var replaceOpt api.GNMIOption
		if useReplaceFiles {
			replaceData, err := readFile(c.LocalFlags.SetReplaceFile[i])
			if err != nil {
				c.logger.Printf("error reading data from file '%s': %v", c.LocalFlags.SetReplaceFile[i], err)
				return nil, err
			}
			trim := ""
			if !c.LocalFlags.SetNoTrim {
				trim = trimChars
			}
			replaceOpt = api.Replace(
				api.Path(strings.TrimSpace(p)),
				api.Value(string(bytes.Trim(replaceData, trim)), c.Encoding),
			)

		} else {
			replaceOpt = api.Replace(
				api.Path(strings.TrimSpace(p)),
				api.Value(c.LocalFlags.SetReplaceValue[i], c.Encoding),
			)
		}
		gnmiOpts = append(gnmiOpts, replaceOpt)
	}

	for i, p := range c.LocalFlags.SetUnionReplacePath {
		var unionReplaceOpt api.GNMIOption
		if useUnionReplaceFiles {
			replaceData, err := readFile(c.LocalFlags.SetUnionReplaceFile[i])
			if err != nil {
				c.logger.Printf("error reading data from file '%s': %v", c.LocalFlags.SetUnionReplaceFile[i], err)
				return nil, err
			}
			trim := ""
			if !c.LocalFlags.SetNoTrim {
				trim = trimChars
			}
			unionReplaceOpt = api.UnionReplace(
				api.Path(strings.TrimSpace(p)),
				api.Value(string(bytes.Trim(replaceData, trim)), c.Encoding),
			)

		} else {
			unionReplaceOpt = api.UnionReplace(
				api.Path(strings.TrimSpace(p)),
				api.Value(c.LocalFlags.SetUnionReplaceValue[i], c.Encoding),
			)
		}
		gnmiOpts = append(gnmiOpts, unionReplaceOpt)
	}

	if len(c.LocalFlags.SetUpdateCli) > 0 {
		gnmiOpts = append(gnmiOpts,
			api.Update(
				api.Path("cli:/"),
				api.Value(strings.Join(c.LocalFlags.SetUpdateCli, "\n"), "ascii"),
			),
		)
	}

	if len(c.LocalFlags.SetReplaceCli) > 0 {
		gnmiOpts = append(gnmiOpts,
			api.Replace(
				api.Path("cli:/"),
				api.Value(strings.Join(c.LocalFlags.SetReplaceCli, "\n"), "ascii"),
			),
		)
	}

	if c.LocalFlags.SetUpdateCliFile != "" {
		data, err := readFile(c.LocalFlags.SetUpdateCliFile)
		if err != nil {
			return nil, err
		}
		gnmiOpts = append(gnmiOpts,
			api.Update(
				api.Path("cli:/"),
				api.Value(string(data), "ascii"),
			),
		)
	}

	if c.LocalFlags.SetReplaceCliFile != "" {
		data, err := readFile(c.LocalFlags.SetReplaceCliFile)
		if err != nil {
			return nil, err
		}
		gnmiOpts = append(gnmiOpts,
			api.Replace(
				api.Path("cli:/"),
				api.Value(string(data), "ascii"),
			),
		)
	}

	if c.LocalFlags.SetCommitId != "" {
		if c.LocalFlags.SetCommitRequest {
			gnmiOpts = append(gnmiOpts,
				api.Extension_CommitRequest(
					c.LocalFlags.SetCommitId,
					c.LocalFlags.SetCommitRollbackDuration,
				))
		} else if c.LocalFlags.SetCommitConfirm {
			gnmiOpts = append(gnmiOpts,
				api.Extension_CommitConfirm(
					c.LocalFlags.SetCommitId,
				))
		} else if c.LocalFlags.SetCommitCancel {
			gnmiOpts = append(gnmiOpts,
				api.Extension_CommitCancel(
					c.LocalFlags.SetCommitId,
				))
		} else {
			gnmiOpts = append(gnmiOpts,
				api.Extension_CommitSetRollbackDuration(
					c.LocalFlags.SetCommitId,
					c.LocalFlags.SetCommitRollbackDuration,
				))
		}
	}
	//
	req, err := api.NewSetRequest(gnmiOpts...)
	return []*gnmi.SetRequest{req}, err
}

// readFile reads a json or yaml file. the the file is .yaml, converts it to json and returns []byte and an error
func readFile(name string) ([]byte, error) {
	data, err := gfile.ReadFile(context.TODO(), name)
	if err != nil {
		return nil, err
	}
	switch filepath.Ext(name) {
	default:
		return data, nil
	case ".yaml", ".yml":
		return toJSONBytes(data)
	}
}

func toJSONBytes(data []byte) ([]byte, error) {
	var out interface{}
	var err error
	err = yaml.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}
	newStruct := convert(out)

	b := new(bytes.Buffer)
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)

	err = enc.Encode(newStruct)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// SanitizeArrayFlagValue trims trailing and leading brackets ([]),
// from each of ls elements only if both are present.
func SanitizeArrayFlagValue(ls []string) []string {
	res := make([]string, 0, len(ls))
	for i := range ls {
		if ls[i] == "[]" {
			continue
		}
		for strings.HasPrefix(ls[i], "[") && strings.HasSuffix(ls[i], "]") {
			ls[i] = ls[i][1 : len(ls[i])-1]
		}
		res = append(res, ls[i])
	}
	return res
}

func ParseAddressField(addr []string) []string {
	res := make([]string, 0, len(addr))
	for i := range addr {
		if addr[i] == "[]" {
			continue
		}
		for strings.HasPrefix(addr[i], "[") && strings.HasSuffix(addr[i], "]") {
			addr[i] = addr[i][1 : len(addr[i])-1]
		}
		res = append(res, strings.Split(addr[i], ",")...)
	}
	return res
}

func (c *Config) ValidateSetInput() error {
	var err error
	c.LocalFlags.SetDelete = SanitizeArrayFlagValue(c.LocalFlags.SetDelete)
	c.LocalFlags.SetUpdate = SanitizeArrayFlagValue(c.LocalFlags.SetUpdate)
	c.LocalFlags.SetReplace = SanitizeArrayFlagValue(c.LocalFlags.SetReplace)
	c.LocalFlags.SetUpdatePath = SanitizeArrayFlagValue(c.LocalFlags.SetUpdatePath)
	c.LocalFlags.SetReplacePath = SanitizeArrayFlagValue(c.LocalFlags.SetReplacePath)
	c.LocalFlags.SetUpdateValue = SanitizeArrayFlagValue(c.LocalFlags.SetUpdateValue)
	c.LocalFlags.SetReplaceValue = SanitizeArrayFlagValue(c.LocalFlags.SetReplaceValue)
	c.LocalFlags.SetUpdateFile = SanitizeArrayFlagValue(c.LocalFlags.SetUpdateFile)
	c.LocalFlags.SetReplaceFile = SanitizeArrayFlagValue(c.LocalFlags.SetReplaceFile)
	c.LocalFlags.SetRequestFile = SanitizeArrayFlagValue(c.LocalFlags.SetRequestFile)

	c.LocalFlags.SetUpdateFile, err = ExpandOSPaths(c.LocalFlags.SetUpdateFile)
	if err != nil {
		return err
	}
	c.LocalFlags.SetReplaceFile, err = ExpandOSPaths(c.LocalFlags.SetReplaceFile)
	if err != nil {
		return err
	}
	for i := range c.LocalFlags.SetRequestFile {
		c.LocalFlags.SetRequestFile[i], err = expandOSPath(c.LocalFlags.SetRequestFile[i])
		if err != nil {
			return err
		}
	}
	c.LocalFlags.SetRequestVars, err = expandOSPath(c.LocalFlags.SetRequestVars)
	if err != nil {
		return err
	}
	if (len(c.LocalFlags.SetDelete)+len(c.LocalFlags.SetUpdate)+len(c.LocalFlags.SetReplace)+len(c.LocalFlags.SetUnionReplace)) == 0 &&
		(len(c.LocalFlags.SetUpdatePath)+len(c.LocalFlags.SetReplacePath)+len(c.LocalFlags.SetUnionReplacePath)) == 0 &&
		len(c.LocalFlags.SetRequestFile) == 0 &&
		len(c.LocalFlags.SetReplaceCli) == 0 &&
		len(c.LocalFlags.SetUpdateCli) == 0 &&
		len(c.LocalFlags.SetReplaceCliFile) == 0 &&
		len(c.LocalFlags.SetUpdateCliFile) == 0 &&
		len(c.LocalFlags.SetRequestProtoFile) == 0 &&
		c.LocalFlags.SetCommitId == "" {
		return errors.New("no paths or request file provided")
	}
	if len(c.LocalFlags.SetUpdateFile) > 0 && len(c.LocalFlags.SetUpdateValue) > 0 {
		return errors.New("set update from file and value are not supported in the same command")
	}
	if len(c.LocalFlags.SetReplaceFile) > 0 && len(c.LocalFlags.SetReplaceValue) > 0 {
		return errors.New("set replace from file and value are not supported in the same command")
	}
	if len(c.LocalFlags.SetUnionReplaceFile) > 0 && len(c.LocalFlags.SetUnionReplaceValue) > 0 {
		return errors.New("set union-replace from file and value are not supported in the same command")
	}
	if len(c.LocalFlags.SetUpdatePath) != len(c.LocalFlags.SetUpdateValue) && len(c.LocalFlags.SetUpdatePath) != len(c.LocalFlags.SetUpdateFile) {
		return errors.New("missing update value/file or path")
	}
	if len(c.LocalFlags.SetReplacePath) != len(c.LocalFlags.SetReplaceValue) && len(c.LocalFlags.SetReplacePath) != len(c.LocalFlags.SetReplaceFile) {
		return errors.New("missing replace value/file or path")
	}
	if len(c.LocalFlags.SetUnionReplacePath) != len(c.LocalFlags.SetUnionReplaceValue) && len(c.LocalFlags.SetUnionReplacePath) != len(c.LocalFlags.SetUnionReplaceFile) {
		return errors.New("missing union-replace value/file or path")
	}
	return nil
}

func ExpandOSPaths(paths []string) ([]string, error) {
	var err error
	for i := range paths {
		paths[i], err = expandOSPath(paths[i])
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func expandOSPath(p string) (string, error) {
	if p == "-" || p == "" {
		return p, nil
	}
	if strings.HasPrefix(p, "http://") ||
		strings.HasPrefix(p, "https://") ||
		strings.HasPrefix(p, "sftp://") ||
		strings.HasPrefix(p, "ftp://") {
		return p, nil
	}
	np, err := homedir.Expand(p)
	if err != nil {
		return "", fmt.Errorf("path %q: %v", p, err)
	}
	if !filepath.IsAbs(np) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("path %q: %v", p, err)
		}
		np = filepath.Join(cwd, np)
	}
	_, err = os.Stat(np)
	if err != nil {
		return "", err
	}
	return np, nil
}

func (c *Config) expandOSPathFlagValues() error {
	for _, flagName := range osPathFlags {
		if c.FileConfig.IsSet(flagName) {
			expandedPath, err := expandOSPath(c.FileConfig.GetString(flagName))
			if err != nil {
				return err
			}
			c.FileConfig.Set(flagName, expandedPath)
		}
	}
	return nil
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}
