package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/adrg/xdg"
	"github.com/itchyny/gojq"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/mitchellh/go-homedir"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

const (
	configName      = ".gnmic"
	configLogPrefix = "[config] "
	envPrefix       = "GNMIC"
)

var osPathFlags = []string{"tls-ca", "tls-cert", "tls-key"}

type Config struct {
	GlobalFlags `mapstructure:",squash"`
	LocalFlags  `mapstructure:",squash"`
	FileConfig  *viper.Viper `mapstructure:"-" json:"-" yaml:"-" `

	Targets            map[string]*types.TargetConfig       `mapstructure:"targets,omitempty" json:"targets,omitempty" yaml:"targets,omitempty"`
	Subscriptions      map[string]*types.SubscriptionConfig `mapstructure:"subscriptions,omitempty" json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
	Outputs            map[string]map[string]interface{}    `mapstructure:"outputs,omitempty" json:"outputs,omitempty" yaml:"outputs,omitempty"`
	Inputs             map[string]map[string]interface{}    `mapstructure:"inputs,omitempty" json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Processors         map[string]map[string]interface{}    `mapstructure:"processors,omitempty" json:"processors,omitempty" yaml:"processors,omitempty"`
	Clustering         *clustering                          `mapstructure:"clustering,omitempty" json:"clustering,omitempty" yaml:"clustering,omitempty"`
	GnmiServer         *gnmiServer                          `mapstructure:"gnmi-server,omitempty" json:"gnmi-server,omitempty" yaml:"gnmi-server,omitempty"`
	APIServer          *APIServer                           `mapstructure:"api-server,omitempty" json:"api-server,omitempty" yaml:"api-server,omitempty"`
	logger             *log.Logger
	setRequestTemplate *template.Template
	setRequestVars     map[string]interface{}
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
	Timeout       time.Duration `mapstructure:"timeout,omitempty" json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Debug         bool          `mapstructure:"debug,omitempty" json:"debug,omitempty" yaml:"debug,omitempty"`
	SkipVerify    bool          `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty" yaml:"skip-verify,omitempty"`
	NoPrefix      bool          `mapstructure:"no-prefix,omitempty" json:"no-prefix,omitempty" yaml:"no-prefix,omitempty"`
	ProxyFromEnv  bool          `mapstructure:"proxy-from-env,omitempty" json:"proxy-from-env,omitempty" yaml:"proxy-from-env,omitempty"`
	Format        string        `mapstructure:"format,omitempty" json:"format,omitempty" yaml:"format,omitempty"`
	LogFile       string        `mapstructure:"log-file,omitempty" json:"log-file,omitempty" yaml:"log-file,omitempty"`
	Log           bool          `mapstructure:"log,omitempty" json:"log,omitempty" yaml:"log,omitempty"`
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
}

type LocalFlags struct {
	// Capabilities
	CapabilitiesVersion bool `mapstructure:"capabilities-version,omitempty" json:"capabilities-version,omitempty" yaml:"capabilities-version,omitempty"`
	// Get
	GetPath   []string `mapstructure:"get-path,omitempty" json:"get-path,omitempty" yaml:"get-path,omitempty"`
	GetPrefix string   `mapstructure:"get-prefix,omitempty" json:"get-prefix,omitempty" yaml:"get-prefix,omitempty"`
	GetModel  []string `mapstructure:"get-model,omitempty" json:"get-model,omitempty" yaml:"get-model,omitempty"`
	GetType   string   `mapstructure:"get-type,omitempty" json:"get-type,omitempty" yaml:"get-type,omitempty"`
	GetTarget string   `mapstructure:"get-target,omitempty" json:"get-target,omitempty" yaml:"get-target,omitempty"`
	// Set
	SetPrefix       string   `mapstructure:"set-prefix,omitempty" json:"set-prefix,omitempty" yaml:"set-prefix,omitempty"`
	SetDelete       []string `mapstructure:"set-delete,omitempty" json:"set-delete,omitempty" yaml:"set-delete,omitempty"`
	SetReplace      []string `mapstructure:"set-replace,omitempty" json:"set-replace,omitempty" yaml:"set-replace,omitempty"`
	SetUpdate       []string `mapstructure:"set-update,omitempty" json:"set-update,omitempty" yaml:"set-update,omitempty"`
	SetReplacePath  []string `mapstructure:"set-replace-path,omitempty" json:"set-replace-path,omitempty" yaml:"set-replace-path,omitempty"`
	SetUpdatePath   []string `mapstructure:"set-update-path,omitempty" json:"set-update-path,omitempty" yaml:"set-update-path,omitempty"`
	SetReplaceFile  []string `mapstructure:"set-replace-file,omitempty" json:"set-replace-file,omitempty" yaml:"set-replace-file,omitempty"`
	SetUpdateFile   []string `mapstructure:"set-update-file,omitempty" json:"set-update-file,omitempty" yaml:"set-update-file,omitempty"`
	SetReplaceValue []string `mapstructure:"set-replace-value,omitempty" json:"set-replace-value,omitempty" yaml:"set-replace-value,omitempty"`
	SetUpdateValue  []string `mapstructure:"set-update-value,omitempty" json:"set-update-value,omitempty" yaml:"set-update-value,omitempty"`
	SetDelimiter    string   `mapstructure:"set-delimiter,omitempty" json:"set-delimiter,omitempty" yaml:"set-delimiter,omitempty"`
	SetTarget       string   `mapstructure:"set-target,omitempty" json:"set-target,omitempty" yaml:"set-target,omitempty"`
	SetRequestFile  string   `mapstructure:"set-request-file,omitempty" json:"set-request-file,omitempty" yaml:"set-request-file,omitempty"`
	SetRequestVars  string   `mapstructure:"set-request-vars,omitempty" json:"set-request-vars,omitempty" yaml:"set-request-vars,omitempty"`
	// Sub
	SubscribePrefix            string        `mapstructure:"subscribe-prefix,omitempty" json:"subscribe-prefix,omitempty" yaml:"subscribe-prefix,omitempty"`
	SubscribePath              []string      `mapstructure:"subscribe-path,omitempty" json:"subscribe-path,omitempty" yaml:"subscribe-path,omitempty"`
	SubscribeQos               uint32        `mapstructure:"subscribe-qos,omitempty" json:"subscribe-qos,omitempty" yaml:"subscribe-qos,omitempty"`
	SubscribeUpdatesOnly       bool          `mapstructure:"subscribe-updates-only,omitempty" json:"subscribe-updates-only,omitempty" yaml:"subscribe-updates-only,omitempty"`
	SubscribeMode              string        `mapstructure:"subscribe-mode,omitempty" json:"subscribe-mode,omitempty" yaml:"subscribe-mode,omitempty"`
	SubscribeStreamMode        string        `mapstructure:"subscribe-stream_mode,omitempty" json:"subscribe-stream-mode,omitempty" yaml:"subscribe-stream-mode,omitempty"`
	SubscribeSampleInterval    time.Duration `mapstructure:"subscribe-sample-interval,omitempty" json:"subscribe-sample-interval,omitempty" yaml:"subscribe-sample-interval,omitempty"`
	SubscribeSuppressRedundant bool          `mapstructure:"subscribe-suppress-redundant,omitempty" json:"subscribe-suppress-redundant,omitempty" yaml:"subscribe-suppress-redundant,omitempty"`
	SubscribeHeartbearInterval time.Duration `mapstructure:"subscribe-heartbear-interval,omitempty" json:"subscribe-heartbear-interval,omitempty" yaml:"subscribe-heartbear-interval,omitempty"`
	SubscribeModel             []string      `mapstructure:"subscribe-model,omitempty" json:"subscribe-model,omitempty" yaml:"subscribe-model,omitempty"`
	SubscribeQuiet             bool          `mapstructure:"subscribe-quiet,omitempty" json:"subscribe-quiet,omitempty" yaml:"subscribe-quiet,omitempty"`
	SubscribeTarget            string        `mapstructure:"subscribe-target,omitempty" json:"subscribe-target,omitempty" yaml:"subscribe-target,omitempty"`
	SubscribeSetTarget         bool          `mapstructure:"subscribe-set-target,omitempty" json:"subscribe-set-target,omitempty" yaml:"subscribe-set-target,omitempty"`
	SubscribeName              []string      `mapstructure:"subscribe-name,omitempty" json:"subscribe-name,omitempty" yaml:"subscribe-name,omitempty"`
	SubscribeOutput            []string      `mapstructure:"subscribe-output,omitempty" json:"subscribe-output,omitempty" yaml:"subscribe-output,omitempty"`
	SubscribeWatchConfig       bool          `mapstructure:"subscribe-watch-config,omitempty" json:"subscribe-watch-config,omitempty" yaml:"subscribe-watch-config,omitempty"`
	SubscribeBackoff           time.Duration `mapstructure:"subscribe-backoff,omitempty" json:"subscribe-backoff,omitempty" yaml:"subscribe-backoff,omitempty"`

	SubscribeLockRetry time.Duration `mapstructure:"subscribe-lock-retry,omitempty" json:"subscribe-lock-retry,omitempty" yaml:"subscribe-lock-retry,omitempty"`
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
	GetSetPrefix    string `mapstructure:"getset-prefix,omitempty" json:"getset-prefix,omitempty" yaml:"getset-prefix,omitempty"`
	GetSetGet       string `mapstructure:"getset-get,omitempty" json:"getset-get,omitempty" yaml:"getset-get,omitempty"`
	GetSetModel     []string
	GetSetTarget    string `mapstructure:"getset-target,omitempty" json:"getset-target,omitempty" yaml:"getset-target,omitempty"`
	GetSetType      string `mapstructure:"getset-type,omitempty" json:"getset-type,omitempty" yaml:"getset-type,omitempty"`
	GetSetCondition string `mapstructure:"getset-condition,omitempty" json:"getset-condition,omitempty" yaml:"getset-condition,omitempty"`
	GetSetUpdate    string `mapstructure:"getset-update,omitempty" json:"getset-update,omitempty" yaml:"getset-update,omitempty"`
	GetSetReplace   string `mapstructure:"getset-replace,omitempty" json:"getset-replace,omitempty" yaml:"getset-replace,omitempty"`
	GetSetDelete    string `mapstructure:"getset-delete,omitempty" json:"getset-delete,omitempty" yaml:"getset-delete,omitempty"`
	GetSetValue     string `mapstructure:"getset-value,omitempty" json:"getset-value,omitempty" yaml:"getset-value,omitempty"`
	// Generate
	GenerateOutput     string `mapstructure:"generate-output,omitempty" json:"generate-output,omitempty" yaml:"generate-output,omitempty"`
	GenerateJSON       bool   `mapstructure:"generate-json,omitempty" json:"generate-json,omitempty" yaml:"generate-json,omitempty"`
	GenerateConfigOnly bool   `mapstructure:"generate-config-only,omitempty" json:"generate-config-only,omitempty" yaml:"generate-config-only,omitempty"`
	GeneratePath       string `mapstructure:"generate-path,omitempty" json:"generate-path,omitempty" yaml:"generate-path,omitempty"`
	// Generate Set Request
	GenerateSetRequestUpdatePath  []string `mapstructure:"generate-update-path,omitempty" json:"generate-update-path,omitempty" yaml:"generate-update-path,omitempty"`
	GenerateSetRequestReplacePath []string `mapstructure:"generate-replace-path,omitempty" json:"generate-replace-path,omitempty" yaml:"generate-replace-path,omitempty"`
	// Generate path
	GeneratePathWithDescr  bool   `mapstructure:"generate-descr,omitempty" json:"generate-descr,omitempty" yaml:"generate-descr,omitempty"`
	GeneratePathWithPrefix bool   `mapstructure:"generate-with-prefix,omitempty" json:"generate-with-prefix,omitempty" yaml:"generate-with-prefix,omitempty"`
	GeneratePathWithTypes  bool   `mapstructure:"generate-types,omitempty" json:"generate-types,omitempty" yaml:"generate-types,omitempty"`
	GeneratePathSearch     bool   `mapstructure:"generate-search,omitempty" json:"generate-search,omitempty" yaml:"generate-search,omitempty"`
	GeneratePathPathType   string `mapstructure:"generate-path-path-type,omitempty" json:"generate-path-path-type,omitempty" yaml:"generate-path-path-type,omitempty"`
	GeneratePathState      bool   `mapstructure:"generate-path-state,omitempty" json:"generate-path-state,omitempty" yaml:"generate-path-state,omitempty"`
	GeneratePathConfig     bool   `mapstructure:"generate-path-config,omitempty" json:"generate-path-config,omitempty" yaml:"generate-path-config,omitempty"`
	//
	DiffPath    []string `mapstructure:"diff-path,omitempty" json:"diff-path,omitempty" yaml:"diff-path,omitempty"`
	DiffPrefix  string   `mapstructure:"diff-prefix,omitempty" json:"diff-prefix,omitempty" yaml:"diff-prefix,omitempty"`
	DiffModel   []string `mapstructure:"diff-model,omitempty" json:"diff-model,omitempty" yaml:"diff-model,omitempty"`
	DiffType    string   `mapstructure:"diff-type,omitempty" json:"diff-type,omitempty" yaml:"diff-type,omitempty"`
	DiffTarget  string   `mapstructure:"diff-target,omitempty" json:"diff-target,omitempty" yaml:"diff-target,omitempty"`
	DiffSub     bool     `mapstructure:"diff-sub,omitempty" json:"diff-sub,omitempty" yaml:"diff-sub,omitempty"`
	DiffRef     string   `mapstructure:"diff-ref,omitempty" json:"diff-ref,omitempty" yaml:"diff-ref,omitempty"`
	DiffCompare []string `mapstructure:"diff-compare,omitempty" json:"diff-compare,omitempty" yaml:"diff-compare,omitempty"`
	DiffQos     uint32   `mapstructure:"diff-qos,omitempty" json:"diff-qos,omitempty" yaml:"diff-qos,omitempty"`
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
		log.New(ioutil.Discard, configLogPrefix, log.LstdFlags|log.Lmicroseconds|log.Lmsgprefix),
		nil,
		make(map[string]interface{}),
	}
}

func (c *Config) Load() error {
	c.FileConfig.SetEnvPrefix(envPrefix)
	c.FileConfig.SetEnvKeyReplacer(strings.NewReplacer("/", "_", "-", "_"))
	c.FileConfig.AutomaticEnv()
	if c.GlobalFlags.CfgFile != "" {
		c.FileConfig.SetConfigFile(c.GlobalFlags.CfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			return err
		}
		c.FileConfig.AddConfigPath(".")
		c.FileConfig.AddConfigPath(home)
		c.FileConfig.AddConfigPath(xdg.ConfigHome)
		c.FileConfig.AddConfigPath(xdg.ConfigHome + "/gnmic")
		c.FileConfig.SetConfigName(configName)
	}

	err := c.FileConfig.ReadInConfig()
	if err != nil {
		return err
	}

	err = c.FileConfig.Unmarshal(c.FileConfig)
	if err != nil {
		return err
	}
	c.mergeEnvVars()
	return c.expandOSPathFlagValues()
}

func (c *Config) SetLogger() (io.Writer, int, error) {
	var f io.Writer = ioutil.Discard
	var loggingFlags = c.logger.Flags()
	var err error

	if c.LogFile != "" {
		f, err = os.OpenFile(c.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return nil, 0, err
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
		loggingFlags = loggingFlags | log.Llongfile
	}
	c.logger.SetOutput(f)
	c.logger.SetFlags(loggingFlags)
	return f, loggingFlags, nil
}

func (c *Config) SetPersistantFlagsFromFile(cmd *cobra.Command) {
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
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

func (c *Config) CreateGetRequest() (*gnmi.GetRequest, error) {
	if c == nil {
		return nil, errors.New("invalid configuration")
	}
	encodingVal, ok := gnmi.Encoding_value[strings.Replace(strings.ToUpper(c.Encoding), "-", "_", -1)]
	if !ok {
		return nil, fmt.Errorf("invalid encoding type '%s'", c.Encoding)
	}
	req := &gnmi.GetRequest{
		UseModels: make([]*gnmi.ModelData, 0),
		Path:      make([]*gnmi.Path, 0, len(c.LocalFlags.GetPath)),
		Encoding:  gnmi.Encoding(encodingVal),
	}
	if c.LocalFlags.GetPrefix != "" {
		gnmiPrefix, err := utils.ParsePath(c.LocalFlags.GetPrefix)
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
		req.Prefix = gnmiPrefix
	}
	if c.LocalFlags.GetTarget != "" {
		if req.Prefix == nil {
			req.Prefix = &gnmi.Path{}
		}
		req.Prefix.Target = c.LocalFlags.GetTarget
	}
	if c.LocalFlags.GetType != "" {
		dti, ok := gnmi.GetRequest_DataType_value[strings.ToUpper(c.LocalFlags.GetType)]
		if !ok {
			return nil, fmt.Errorf("unknown data type %s", c.LocalFlags.GetType)
		}
		req.Type = gnmi.GetRequest_DataType(dti)
	}
	for _, p := range c.LocalFlags.GetPath {
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		req.Path = append(req.Path, gnmiPath)
	}
	return req, nil
}

func (c *Config) CreateGASGetRequest() (*gnmi.GetRequest, error) {
	if c == nil {
		return nil, errors.New("invalid configuration")
	}
	encodingVal, ok := gnmi.Encoding_value[strings.Replace(strings.ToUpper(c.Encoding), "-", "_", -1)]
	if !ok {
		return nil, fmt.Errorf("invalid encoding type '%s'", c.Encoding)
	}
	req := &gnmi.GetRequest{
		UseModels: make([]*gnmi.ModelData, 0),
		Path:      make([]*gnmi.Path, 0, 1),
		Encoding:  gnmi.Encoding(encodingVal),
	}
	if c.LocalFlags.GetSetPrefix != "" {
		gnmiPrefix, err := utils.ParsePath(c.LocalFlags.GetSetPrefix)
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
		req.Prefix = gnmiPrefix
	}
	if c.LocalFlags.GetSetTarget != "" {
		if req.Prefix == nil {
			req.Prefix = &gnmi.Path{}
		}
		req.Prefix.Target = c.LocalFlags.GetSetTarget
	}
	if c.LocalFlags.GetSetType != "" {
		dti, ok := gnmi.GetRequest_DataType_value[strings.ToUpper(c.LocalFlags.GetSetType)]
		if !ok {
			return nil, fmt.Errorf("unknown data type %s", c.LocalFlags.GetSetType)
		}
		req.Type = gnmi.GetRequest_DataType(dti)
	}

	gnmiPath, err := utils.ParsePath(strings.TrimSpace(c.LocalFlags.GetSetGet))
	if err != nil {
		return nil, fmt.Errorf("path parse error: %v", err)
	}
	req.Path = append(req.Path, gnmiPath)
	return req, nil
}

func (c *Config) CreateGASSetRequest(input interface{}) (*gnmi.SetRequest, error) {
	gnmiPrefix, err := utils.CreatePrefix(c.LocalFlags.GetSetPrefix, c.LocalFlags.GetSetTarget)
	if err != nil {
		return nil, fmt.Errorf("prefix parse error: %v", err)
	}
	req := &gnmi.SetRequest{
		Prefix:  gnmiPrefix,
		Delete:  make([]*gnmi.Path, 0, 1),
		Replace: make([]*gnmi.Update, 0, 1),
		Update:  make([]*gnmi.Update, 0, 1),
	}
	delPath, err := c.execPathTemplate(c.LocalFlags.GetSetDelete, input)
	if err != nil {
		return nil, err
	}
	if delPath != nil {
		req.Delete = append(req.Delete, delPath)
	}
	updatePath, err := c.execPathTemplate(c.LocalFlags.GetSetUpdate, input)
	if err != nil {
		return nil, err
	}
	replacePath, err := c.execPathTemplate(c.LocalFlags.GetSetReplace, input)
	if err != nil {
		return nil, err
	}
	val, err := c.execValueTemplate(c.LocalFlags.GetSetValue, c.Encoding, input)
	if err != nil {
		return nil, err
	}
	if updatePath != nil {
		req.Update = append(req.Update, &gnmi.Update{
			Path: updatePath,
			Val:  val,
		})
	} else if replacePath != nil {
		req.Replace = append(req.Replace, &gnmi.Update{
			Path: replacePath,
			Val:  val,
		})
	}
	return req, nil
}

func (c *Config) execPathTemplate(tplString string, input interface{}) (*gnmi.Path, error) {
	if tplString == "" {
		return nil, nil
	}
	tplString = os.ExpandEnv(tplString)
	q, err := gojq.Parse(tplString)
	if err != nil {
		return nil, err
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("unexpected jq result type: %T", res)
	}
	switch v := res.(type) {
	case error:
		return nil, v
	case string:
		c.logger.Printf("path jq expression result: %s", v)
		return utils.ParsePath(v)
	default:
		if c.Debug {
			c.logger.Printf("jq input: %+v", input)
			c.logger.Printf("jq result: %+v", v)
		}
		return nil, fmt.Errorf("unexpected jq result type: %T", v)
	}
}

func (c *Config) execValueTemplate(tplString string, encoding string, input interface{}) (*gnmi.TypedValue, error) {
	if tplString == "" {
		return nil, nil
	}
	tplString = os.ExpandEnv(tplString)
	q, err := gojq.Parse(tplString)
	if err != nil {
		return nil, err
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("unexpected jq result type: %T", res)
	}
	switch v := res.(type) {
	case error:
		return nil, v
	case string:
		c.logger.Printf("path jq expression result: %s", v)
		value := new(gnmi.TypedValue)
		err = setValue(value, encoding, v)
		return value, err
	default:
		if c.Debug {
			c.logger.Printf("jq input: %+v", input)
			c.logger.Printf("jq result: %+v", v)
		}
		return nil, fmt.Errorf("unexpected jq result type: %T", v)
	}
}

func (c *Config) CreateSetRequest(targetName string) (*gnmi.SetRequest, error) {
	if c.SetRequestFile != "" {
		return c.CreateSetRequestFromFile(targetName)
	}
	gnmiPrefix, err := utils.CreatePrefix(c.LocalFlags.SetPrefix, c.LocalFlags.SetTarget)
	if err != nil {
		return nil, fmt.Errorf("prefix parse error: %v", err)
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
	}

	//
	useUpdateFiles := len(c.LocalFlags.SetUpdateFile) > 0 && len(c.LocalFlags.SetUpdateValue) == 0
	useReplaceFiles := len(c.LocalFlags.SetReplaceFile) > 0 && len(c.LocalFlags.SetReplaceValue) == 0
	req := &gnmi.SetRequest{
		Prefix:  gnmiPrefix,
		Delete:  make([]*gnmi.Path, 0, len(c.LocalFlags.SetDelete)),
		Replace: make([]*gnmi.Update, 0),
		Update:  make([]*gnmi.Update, 0),
	}
	for _, p := range c.LocalFlags.SetDelete {
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		req.Delete = append(req.Delete, gnmiPath)
	}
	for _, u := range c.LocalFlags.SetUpdate {
		singleUpdate := strings.Split(u, c.LocalFlags.SetDelimiter)
		if len(singleUpdate) < 3 {
			return nil, fmt.Errorf("invalid inline update format: %s", c.LocalFlags.SetUpdate)
		}
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(singleUpdate[0]))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		err = setValue(value, singleUpdate[1], singleUpdate[2])
		if err != nil {
			return nil, err
		}
		req.Update = append(req.Update, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for _, r := range c.LocalFlags.SetReplace {
		singleReplace := strings.Split(r, c.LocalFlags.SetDelimiter)
		if len(singleReplace) < 3 {
			return nil, fmt.Errorf("invalid inline replace format: %s", c.LocalFlags.SetReplace)
		}
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(singleReplace[0]))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		err = setValue(value, singleReplace[1], singleReplace[2])
		if err != nil {
			return nil, err
		}
		req.Replace = append(req.Replace, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for i, p := range c.LocalFlags.SetUpdatePath {
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		if useUpdateFiles {
			var updateData []byte
			updateData, err = readFile(c.LocalFlags.SetUpdateFile[i])
			if err != nil {
				c.logger.Printf("error reading data from file '%s': %v", c.LocalFlags.SetUpdateFile[i], err)
				return nil, err
			}
			switch strings.ToUpper(c.Encoding) {
			case "JSON":
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(updateData, " \r\n\t"),
				}
			case "JSON_IETF":
				value.Value = &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(updateData, " \r\n\t"),
				}
			default:
				return nil, fmt.Errorf("encoding: %q not supported together with file values", c.Encoding)
			}
		} else {
			err = setValue(value, strings.ToLower(c.Encoding), c.LocalFlags.SetUpdateValue[i])
			if err != nil {
				return nil, err
			}
		}
		req.Update = append(req.Update, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for i, p := range c.LocalFlags.SetReplacePath {
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		if useReplaceFiles {
			var replaceData []byte
			replaceData, err = readFile(c.LocalFlags.SetReplaceFile[i])
			if err != nil {
				c.logger.Printf("error reading data from file '%s': %v", c.LocalFlags.SetReplaceFile[i], err)
				return nil, err
			}
			switch strings.ToUpper(c.Encoding) {
			case "JSON":
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(replaceData, " \r\n\t"),
				}
			case "JSON_IETF":
				value.Value = &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(replaceData, " \r\n\t"),
				}
			default:
				return nil, fmt.Errorf("encoding: %q not supported together with file values", c.Encoding)
			}
		} else {
			err = setValue(value, strings.ToLower(c.Encoding), c.LocalFlags.SetReplaceValue[i])
			if err != nil {
				return nil, err
			}
		}
		req.Replace = append(req.Replace, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	return req, nil
}

func setValue(value *gnmi.TypedValue, typ, val string) error {
	var err error
	switch typ {
	case "json":
		val = strings.TrimRight(strings.TrimLeft(val, "["), "]")
		buff := new(bytes.Buffer)
		bval := json.RawMessage(val)
		if json.Valid(bval) {
			err = json.NewEncoder(buff).Encode(bval)
		} else {
			err = json.NewEncoder(buff).Encode(val)
		}
		if err != nil {
			return err
		}

		value.Value = &gnmi.TypedValue_JsonVal{
			JsonVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
		}
	case "json_ietf":
		val = strings.TrimRight(strings.TrimLeft(val, "["), "]")
		buff := new(bytes.Buffer)
		bval := json.RawMessage(val)
		if json.Valid(bval) {
			err = json.NewEncoder(buff).Encode(bval)
		} else {
			err = json.NewEncoder(buff).Encode(val)
		}
		if err != nil {
			return err
		}

		value.Value = &gnmi.TypedValue_JsonIetfVal{
			JsonIetfVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
		}
	case "ascii":
		value.Value = &gnmi.TypedValue_AsciiVal{
			AsciiVal: trimQuotes(val),
		}
	case "bool":
		bval, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_BoolVal{
			BoolVal: bval,
		}
	case "bytes":
		value.Value = &gnmi.TypedValue_BytesVal{
			BytesVal: []byte(val),
		}
	case "decimal":
		dVal := &gnmi.Decimal64{}
		value.Value = &gnmi.TypedValue_DecimalVal{
			DecimalVal: dVal,
		}
		return fmt.Errorf("decimal type not implemented")
	case "float":
		f, err := strconv.ParseFloat(val, 32)
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_FloatVal{
			FloatVal: float32(f),
		}
	case "int":
		k, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_IntVal{
			IntVal: k,
		}
	case "uint":
		u, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_UintVal{
			UintVal: u,
		}
	case "string":
		value.Value = &gnmi.TypedValue_StringVal{
			StringVal: trimQuotes(val),
		}
	default:
		return fmt.Errorf("unknown type %q, must be one of: %v", typ, ValueTypes)
	}
	return nil
}

// readFile reads a json or yaml file. the the file is .yaml, converts it to json and returns []byte and an error
func readFile(name string) ([]byte, error) {
	var in io.Reader
	var err error
	var data []byte

	if name == "-" {
		in = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
			return nil, err
		}
		data = make([]byte, 0, st.Size())
		in = f
	}

	sc := bufio.NewScanner(in)
	sc.Split(bufio.ScanBytes)

	for sc.Scan() {
		data = append(data, sc.Bytes()...)
	}
	//
	switch filepath.Ext(name) {
	case ".json":
		return data, err
	case ".yaml", ".yml":
		return tryYAML(data)
	default:
		// try yaml
		newData, err := tryYAML(data)
		if err != nil {
			// assume json
			return data, nil
		}
		return newData, nil
	}
}

func tryYAML(data []byte) ([]byte, error) {
	var out interface{}
	var err error
	err = yaml.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}
	newStruct := convert(out)
	newData, err := json.Marshal(newStruct)
	if err != nil {
		return nil, err
	}
	return newData, nil
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
		res = append(res, strings.Split(ls[i], ",")...)
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

	c.LocalFlags.SetUpdateFile, err = ExpandOSPaths(c.LocalFlags.SetUpdateFile)
	if err != nil {
		return err
	}
	c.LocalFlags.SetReplaceFile, err = ExpandOSPaths(c.LocalFlags.SetReplaceFile)
	if err != nil {
		return err
	}
	c.LocalFlags.SetRequestFile, err = expandOSPath(c.LocalFlags.SetRequestFile)
	if err != nil {
		return err
	}
	c.LocalFlags.SetRequestVars, err = expandOSPath(c.LocalFlags.SetRequestVars)
	if err != nil {
		return err
	}
	if (len(c.LocalFlags.SetDelete)+len(c.LocalFlags.SetUpdate)+len(c.LocalFlags.SetReplace)) == 0 &&
		(len(c.LocalFlags.SetUpdatePath)+len(c.LocalFlags.SetReplacePath)) == 0 &&
		c.LocalFlags.SetRequestFile == "" {
		return errors.New("no paths or request file provided")
	}
	if len(c.LocalFlags.SetUpdateFile) > 0 && len(c.LocalFlags.SetUpdateValue) > 0 {
		return errors.New("set update from file and value are not supported in the same command")
	}
	if len(c.LocalFlags.SetReplaceFile) > 0 && len(c.LocalFlags.SetReplaceValue) > 0 {
		return errors.New("set replace from file and value are not supported in the same command")
	}
	if len(c.LocalFlags.SetUpdatePath) != len(c.LocalFlags.SetUpdateValue) && len(c.LocalFlags.SetUpdatePath) != len(c.LocalFlags.SetUpdateFile) {
		return errors.New("missing update value/file or path")
	}
	if len(c.LocalFlags.SetReplacePath) != len(c.LocalFlags.SetReplaceValue) && len(c.LocalFlags.SetReplacePath) != len(c.LocalFlags.SetReplaceFile) {
		return errors.New("missing replace value/file or path")
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
