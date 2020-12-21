package config

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	configPath      = ""
	configName      = "gnmic"
	configLogPrefix = "config "
)

type Config struct {
	Globals    *GlobalFlags
	LocalFlags *LocalFlags
	FileConfig *viper.Viper

	logger *log.Logger
}

type GlobalFlags struct {
	Address           []string      `mapstructure:"address,omitempty" json:"address,omitempty" yaml:"address,omitempty"`
	Username          string        `mapstructure:"username,omitempty" json:"username,omitempty" yaml:"username,omitempty"`
	Password          string        `mapstructure:"password,omitempty" json:"password,omitempty" yaml:"password,omitempty"`
	Port              string        `mapstructure:"port,omitempty" json:"port,omitempty" yaml:"port,omitempty"`
	Encoding          string        `mapstructure:"encoding,omitempty" json:"encoding,omitempty" yaml:"encoding,omitempty"`
	Insecure          bool          `mapstructure:"insecure,omitempty" json:"insecure,omitempty" yaml:"insecure,omitempty"`
	TLSCa             string        `mapstructure:"tls-ca,omitempty" json:"tls-ca,omitempty" yaml:"tls-ca,omitempty"`
	TLSCert           string        `mapstructure:"tls-cert,omitempty" json:"tls-cert,omitempty" yaml:"tls-cert,omitempty"`
	TLSKey            string        `mapstructure:"tls-key,omitempty" json:"tls-key,omitempty" yaml:"tls-key,omitempty"`
	TLSMinVersion     string        `mapstructure:"tls-min-version,omitempty" json:"tls-min-version,omitempty" yaml:"tls-min-version,omitempty"`
	TLSMaxVersion     string        `mapstructure:"tls-max-version,omitempty" json:"tls-max-version,omitempty" yaml:"tls-max-version,omitempty"`
	TLSVersion        string        `mapstructure:"tls-version,omitempty" json:"tls-version,omitempty" yaml:"tls-version,omitempty"`
	Timeout           time.Duration `mapstructure:"timeout,omitempty" json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Debug             bool          `mapstructure:"debug,omitempty" json:"debug,omitempty" yaml:"debug,omitempty"`
	SkipVerify        bool          `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty" yaml:"skip-verify,omitempty"`
	NoPrefix          bool          `mapstructure:"no-prefix,omitempty" json:"no-prefix,omitempty" yaml:"no-prefix,omitempty"`
	ProxyFromEnv      bool          `mapstructure:"proxy-from-env,omitempty" json:"proxy-from-env,omitempty" yaml:"proxy-from-env,omitempty"`
	Format            string        `mapstructure:"format,omitempty" json:"format,omitempty" yaml:"format,omitempty"`
	LogFile           string        `mapstructure:"log-file,omitempty" json:"log-file,omitempty" yaml:"log-file,omitempty"`
	Log               bool          `mapstructure:"log,omitempty" json:"log,omitempty" yaml:"log,omitempty"`
	MaxMsgSize        int           `mapstructure:"max-msg-size,omitempty" json:"max-msg-size,omitempty" yaml:"max-msg-size,omitempty"`
	PrometheusAddress string        `mapstructure:"prometheus-address,omitempty" json:"prometheus-address,omitempty" yaml:"prometheus-address,omitempty"`
	PrintRequest      bool          `mapstructure:"print-request,omitempty" json:"print-request,omitempty" yaml:"print-request,omitempty"`
	Retry             time.Duration `mapstructure:"retry,omitempty" json:"retry,omitempty" yaml:"retry,omitempty"`
	TargetBufferSize  uint          `mapstructure:"target-buffer-size,omitempty" json:"target-buffer-size,omitempty" yaml:"target-buffer-size,omitempty"`
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
	// Sub
	SubscribePrefix            string        `mapstructure:"subscribe-prefix,omitempty" json:"subscribe-prefix,omitempty" yaml:"subscribe-prefix,omitempty"`
	SubscribePath              []string      `mapstructure:"subscribe-path,omitempty" json:"subscribe-path,omitempty" yaml:"subscribe-path,omitempty"`
	SubscribeQos               uint32        `mapstructure:"subscribe-qos,omitempty" json:"subscribe-qos,omitempty" yaml:"subscribe-qos,omitempty"`
	SubscribeUpdatesOnly       bool          `mapstructure:"subscribe-updates-only,omitempty" json:"subscribe-updates-only,omitempty" yaml:"subscribe-updates-only,omitempty"`
	SubscribeMode              string        `mapstructure:"subscribe-mode,omitempty" json:"subscribe-mode,omitempty" yaml:"subscribe-mode,omitempty"`
	SubscribeStreamMode        string        `mapstructure:"subscribe-stream_mode,omitempty" json:"subscribe-stream-mode,omitempty" yaml:"subscribe-stream-mode,omitempty"`
	SubscribeSampleInteral     time.Duration `mapstructure:"subscribe-sample-interal,omitempty" json:"subscribe-sample-interal,omitempty" yaml:"subscribe-sample-interal,omitempty"`
	SubscribeSuppressRedundant bool          `mapstructure:"subscribe-suppress-redundant,omitempty" json:"subscribe-suppress-redundant,omitempty" yaml:"subscribe-suppress-redundant,omitempty"`
	SubscribeHeartbearInterval time.Duration `mapstructure:"subscribe-heartbear-interval,omitempty" json:"subscribe-heartbear-interval,omitempty" yaml:"subscribe-heartbear-interval,omitempty"`
	SubscribeModel             []string      `mapstructure:"subscribe-model,omitempty" json:"subscribe-model,omitempty" yaml:"subscribe-model,omitempty"`
	SubscribeQuiet             bool          `mapstructure:"subscribe-quiet,omitempty" json:"subscribe-quiet,omitempty" yaml:"subscribe-quiet,omitempty"`
	SubscribeTarget            string        `mapstructure:"subscribe-target,omitempty" json:"subscribe-target,omitempty" yaml:"subscribe-target,omitempty"`
	SubscribeName              []string      `mapstructure:"subscribe-name,omitempty" json:"subscribe-name,omitempty" yaml:"subscribe-name,omitempty"`
	SubscribeOutput            []string      `mapstructure:"subscribe-output,omitempty" json:"subscribe-output,omitempty" yaml:"subscribe-output,omitempty"`
	// Path
	PathFile       []string `mapstructure:"path-file,omitempty" json:"path-file,omitempty" yaml:"path-file,omitempty"`
	PathExclude    []string `mapstructure:"path-exclude,omitempty" json:"path-exclude,omitempty" yaml:"path-exclude,omitempty"`
	PathDir        []string `mapstructure:"path-dir,omitempty" json:"path-dir,omitempty" yaml:"path-dir,omitempty"`
	PathPathType   string   `mapstructure:"path-path-type,omitempty" json:"path-path-type,omitempty" yaml:"path-path-type,omitempty"`
	PathModule     string   `mapstructure:"path-module,omitempty" json:"path-module,omitempty" yaml:"path-module,omitempty"`
	PathWithPrefix bool     `mapstructure:"path-with-prefix,omitempty" json:"path-with-prefix,omitempty" yaml:"path-with-prefix,omitempty"`
	PathTypes      bool     `mapstructure:"path-types,omitempty" json:"path-types,omitempty" yaml:"path-types,omitempty"`
	PathSearch     bool     `mapstructure:"path-search,omitempty" json:"path-search,omitempty" yaml:"path-search,omitempty"`
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
	// VersionUpgrade
	UpgradeUsePkg bool `mapstructure:"upgrade-use-pkg" json:"upgrade-use-pkg,omitempty" yaml:"upgrade-use-pkg,omitempty"`
}

func New() *Config {
	return &Config{
		Globals: &GlobalFlags{
			Address: make([]string, 0),
		},
		LocalFlags: &LocalFlags{},
		FileConfig: viper.NewWithOptions(viper.KeyDelimiter("/")),
		logger:     log.New(ioutil.Discard, configLogPrefix, log.LstdFlags|log.Lmicroseconds),
	}
}

func (c *Config) Load(file string) error {
	if file != "" {
		c.FileConfig.SetConfigFile(file)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			return err
		}
		c.FileConfig.AddConfigPath(home)
		c.FileConfig.SetConfigName(configName)
	}

	err := c.FileConfig.ReadInConfig()
	if err != nil {
		return err
	}

	err = c.FileConfig.Unmarshal(c.FileConfig)
	if err != nil {
		return nil
	}

	return nil
}

func (c *Config) SetLogger() {
	if c.Globals.LogFile != "" {
		f, err := os.OpenFile(c.Globals.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return
		}
		c.logger.SetOutput(f)
	} else {
		if c.Globals.Debug {
			c.Globals.Log = true
		}
		if c.Globals.Log {
			c.logger.SetOutput(os.Stderr)
		}
	}
	if c.Globals.Debug {
		loggingFlags := c.logger.Flags() | log.Llongfile
		c.logger.SetFlags(loggingFlags)
	}
}

func (c *Config) SetPersistantFlagsFromFile(cmd *cobra.Command) {
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if c.Globals.Debug {
			c.logger.Printf("setting persistant flags from file: cmd=%s, flag=%s, is set: %v", cmd.Name(), f.Name, c.FileConfig.IsSet(f.Name))
		}
		if !f.Changed && c.FileConfig.IsSet(f.Name) {
			if c.Globals.Debug {
				c.logger.Printf("cmd %s, flag %s did not change and is set in file", cmd.Name(), f.Name)
			}
			val := c.FileConfig.Get(f.Name)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func (c *Config) SetLocalFlagsFromFile(cmd *cobra.Command) {
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		flagName := fmt.Sprintf("%s-%s", cmd.Name(), f.Name)
		if c.Globals.Debug {
			c.logger.Printf("setting local flags from file: cmd=%s, flag=%s, is set in file: %v", cmd.Name(), flagName, c.FileConfig.IsSet(flagName))
		}
		if !f.Changed && c.FileConfig.IsSet(flagName) {
			if c.Globals.Debug {
				c.logger.Printf("cmd %s, flag %s did not change and is set in file", cmd.Name(), flagName)
			}
			val := c.FileConfig.Get(flagName)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
