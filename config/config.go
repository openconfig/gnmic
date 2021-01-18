package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/mitchellh/go-homedir"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const (
	configName      = "gnmic"
	configLogPrefix = "config "
)

type Config struct {
	Globals    *GlobalFlags
	LocalFlags *LocalFlags
	FileConfig *viper.Viper

	logger *log.Logger
}

var ValueTypes = []string{"json", "json_ietf", "string", "int", "uint", "bool", "decimal", "float", "bytes", "ascii"}

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
	SubscribeSampleInterval    time.Duration `mapstructure:"subscribe-sample-interval,omitempty" json:"subscribe-sample-interval,omitempty" yaml:"subscribe-sample-interval,omitempty"`
	SubscribeSuppressRedundant bool          `mapstructure:"subscribe-suppress-redundant,omitempty" json:"subscribe-suppress-redundant,omitempty" yaml:"subscribe-suppress-redundant,omitempty"`
	SubscribeHeartbearInterval time.Duration `mapstructure:"subscribe-heartbear-interval,omitempty" json:"subscribe-heartbear-interval,omitempty" yaml:"subscribe-heartbear-interval,omitempty"`
	SubscribeModel             []string      `mapstructure:"subscribe-model,omitempty" json:"subscribe-model,omitempty" yaml:"subscribe-model,omitempty"`
	SubscribeQuiet             bool          `mapstructure:"subscribe-quiet,omitempty" json:"subscribe-quiet,omitempty" yaml:"subscribe-quiet,omitempty"`
	SubscribeTarget            string        `mapstructure:"subscribe-target,omitempty" json:"subscribe-target,omitempty" yaml:"subscribe-target,omitempty"`
	SubscribeName              []string      `mapstructure:"subscribe-name,omitempty" json:"subscribe-name,omitempty" yaml:"subscribe-name,omitempty"`
	SubscribeOutput            []string      `mapstructure:"subscribe-output,omitempty" json:"subscribe-output,omitempty" yaml:"subscribe-output,omitempty"`
	SubscribeWatchConfig       bool          `mapstructure:"subscribe-watch-config,omitempty" json:"subscribe-watch-config,omitempty" yaml:"subscribe-watch-config,omitempty"`
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
		return err
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
			c.logger.Printf("persistent-flag=%s cmd=%s, changed: %v, is set: %v", f.Name, cmd.Name(), f.Changed, c.FileConfig.IsSet(f.Name))
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
			c.logger.Printf("local-flag=%s, cmd=%s, changed=%v, is set: %v", flagName, cmd.Name(), f.Changed, c.FileConfig.IsSet(flagName))
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
	if c == nil || c.Globals == nil || c.LocalFlags == nil {
		return nil, errors.New("invalid configuration")
	}
	encodingVal, ok := gnmi.Encoding_value[strings.Replace(strings.ToUpper(c.Globals.Encoding), "-", "_", -1)]
	if !ok {
		return nil, fmt.Errorf("invalid encoding type '%s'", c.Globals.Encoding)
	}
	req := &gnmi.GetRequest{
		UseModels: make([]*gnmi.ModelData, 0),
		Path:      make([]*gnmi.Path, 0, len(c.LocalFlags.GetPath)),
		Encoding:  gnmi.Encoding(encodingVal),
	}
	if c.LocalFlags.GetPrefix != "" {
		gnmiPrefix, err := collector.ParsePath(c.LocalFlags.GetPrefix)
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
		req.Prefix = gnmiPrefix
	}
	if c.LocalFlags.GetType != "" {
		dti, ok := gnmi.GetRequest_DataType_value[strings.ToUpper(c.LocalFlags.GetType)]
		if !ok {
			return nil, fmt.Errorf("unknown data type %s", c.LocalFlags.GetType)
		}
		req.Type = gnmi.GetRequest_DataType(dti)
	}
	for _, p := range c.LocalFlags.GetPath {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		req.Path = append(req.Path, gnmiPath)
	}
	return req, nil
}

func (c *Config) CreateSetRequest() (*gnmi.SetRequest, error) {
	gnmiPrefix, err := collector.CreatePrefix(c.LocalFlags.SetPrefix, c.LocalFlags.SetTarget)
	if err != nil {
		return nil, fmt.Errorf("prefix parse error: %v", err)
	}
	if c.Globals.Debug {
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
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
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
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(singleUpdate[0]))
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
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(singleReplace[0]))
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
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
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
			switch strings.ToUpper(c.Globals.Encoding) {
			case "JSON":
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(updateData, " \r\n\t"),
				}
			case "JSON_IETF":
				value.Value = &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(updateData, " \r\n\t"),
				}
			default:
				return nil, fmt.Errorf("encoding: %s not supported together with file values", c.Globals.Encoding)
			}
		} else {
			err = setValue(value, strings.ToLower(c.Globals.Encoding), c.LocalFlags.SetUpdateValue[i])
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
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
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
			switch strings.ToUpper(c.Globals.Encoding) {
			case "JSON":
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(replaceData, " \r\n\t"),
				}
			case "JSON_IETF":
				value.Value = &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(replaceData, " \r\n\t"),
				}
			default:
				return nil, fmt.Errorf("encoding: %s not supported together with file values", c.Globals.Encoding)
			}
		} else {
			err = setValue(value, strings.ToLower(c.Globals.Encoding), c.LocalFlags.SetReplaceValue[i])
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
		buff := new(bytes.Buffer)
		err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(val, "["), "]"))
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_JsonVal{
			JsonVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
		}
	case "json_ietf":
		buff := new(bytes.Buffer)
		err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(val, "["), "]"))
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_JsonIetfVal{
			JsonIetfVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
		}
	case "ascii":
		value.Value = &gnmi.TypedValue_AsciiVal{
			AsciiVal: val,
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
			StringVal: val,
		}
	default:
		return fmt.Errorf("unknown type '%s', must be one of: %v", typ, ValueTypes)
	}
	return nil
}

// readFile reads a json or yaml file. the the file is .yaml, converts it to json and returns []byte and an error
func readFile(name string) ([]byte, error) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}
	switch filepath.Ext(name) {
	case ".json":
		return data, err
	case ".yaml", ".yml":
		var out interface{}
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
	default:
		return nil, fmt.Errorf("unsupported file format %s", filepath.Ext(name))
	}
}

// SanitizeArrayFlagValue trims trailing and leading brackets ([]),
// from each of ls elements only if both are present.
func SanitizeArrayFlagValue(ls []string) []string {
	res := make([]string, 0, len(ls))
	for i := range ls {
		if strings.HasPrefix(ls[i], "[") && strings.HasSuffix(ls[i], "]") {
			ls[i] = strings.Trim(ls[i], "[]")
			res = append(res, strings.Split(ls[i], " ")...)
			continue
		}
		res = append(res, ls[i])
	}
	return res
}

func (c *Config) ValidateSetInput() error {
	c.LocalFlags.SetDelete = SanitizeArrayFlagValue(c.LocalFlags.SetDelete)
	c.LocalFlags.SetUpdate = SanitizeArrayFlagValue(c.LocalFlags.SetUpdate)
	c.LocalFlags.SetReplace = SanitizeArrayFlagValue(c.LocalFlags.SetReplace)
	c.LocalFlags.SetUpdatePath = SanitizeArrayFlagValue(c.LocalFlags.SetUpdatePath)
	c.LocalFlags.SetReplacePath = SanitizeArrayFlagValue(c.LocalFlags.SetReplacePath)
	c.LocalFlags.SetUpdateValue = SanitizeArrayFlagValue(c.LocalFlags.SetUpdateValue)
	c.LocalFlags.SetReplaceValue = SanitizeArrayFlagValue(c.LocalFlags.SetReplaceValue)
	c.LocalFlags.SetUpdateFile = SanitizeArrayFlagValue(c.LocalFlags.SetUpdateFile)
	c.LocalFlags.SetReplaceFile = SanitizeArrayFlagValue(c.LocalFlags.SetReplaceFile)
	if (len(c.LocalFlags.SetDelete)+len(c.LocalFlags.SetUpdate)+len(c.LocalFlags.SetReplace)) == 0 && (len(c.LocalFlags.SetUpdatePath)+len(c.LocalFlags.SetReplacePath)) == 0 {
		return errors.New("no paths provided")
	}
	if len(c.LocalFlags.SetUpdateFile) > 0 && len(c.LocalFlags.SetUpdateValue) > 0 {
		fmt.Println(len(c.LocalFlags.SetUpdateFile))
		fmt.Println(len(c.LocalFlags.SetUpdateValue))
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
