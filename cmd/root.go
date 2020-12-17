// Copyright © 2020 Karim Radhouani <medkarimrdi@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/formatters"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
)

const (
	defaultGrpcPort = "57400"
)
const (
	msgSize = 512 * 1024 * 1024
)

var encodingNames = []string{
	"json",
	"bytes",
	"proto",
	"ascii",
	"json_ietf",
}
var encodings = [][2]string{
	{"json", "JSON encoded string (RFC7159)"},
	{"bytes", "byte sequence whose semantics is opaque to the protocol"},
	{"proto", "serialised protobuf message using protobuf.Any"},
	{"ascii", "ASCII encoded string representing text formatted according to a target-defined convention"},
	{"json_ietf", "JSON_IETF encoded string (RFC7951)"},
}
var formatNames = []string{
	"json",
	"protojson",
	"prototext",
	"event",
	"proto",
}
var formats = [][2]string{
	{"json", "similar to protojson but with xpath style paths and decoded timestamps"},
	{"protojson", "protocol buffer messages in JSON format"},
	{"prototext", "protocol buffer messages in textproto format"},
	{"event", "protocol buffer messages as a timestamped list of tags and values"},
	{"proto", "protocol buffer messages in binary wire format"},
}
var tlsVersions = []string{"1.3", "1.2", "1.1", "1.0", "1"}

var cfgFile string
var f io.WriteCloser
var logger *log.Logger
var coll *collector.Collector
var cfg = config.New()

func rootCmdPersistentPreRunE(cmd *cobra.Command, args []string) error {
	debug := viper.GetBool("debug")
	if viper.GetString("log-file") != "" {
		var err error
		f, err = os.OpenFile(viper.GetString("log-file"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("error opening log file: %v", err)
		}
	} else {
		if debug {
			viper.Set("log", true)
		}
		switch viper.GetBool("log") {
		case true:
			f = os.Stderr
		case false:
			f = myWriteCloser{ioutil.Discard}
		}
	}
	loggingFlags := log.LstdFlags | log.Lmicroseconds
	if debug {
		loggingFlags |= log.Llongfile
	}
	logger = log.New(f, "gnmic ", loggingFlags)
	if debug {
		grpclog.SetLogger(logger) //lint:ignore SA1019 see https://github.com/karimra/gnmic/issues/59
		log.Printf("version=%s, commit=%s, date=%s, gitURL=%s, docs=https://gnmic.kmrd.dev", version, commit, date, gitURL)
	}
	cfgFile := viper.ConfigFileUsed()
	if len(cfgFile) != 0 {
		logger.Printf("using config file %s", cfgFile)
		b, err := ioutil.ReadFile(cfgFile)
		if err != nil {
			if cmd.Flag("config").Changed {
				return err
			}
			logger.Printf("failed reading config file: %v", err)
		}
		if debug {
			logger.Printf("config file:\n%s", string(b))
		}
	}
	logConfigKeysValues()
	return nil
}

func rootCmdPersistentPostRun(cmd *cobra.Command, args []string) {
	if !viper.GetBool("log") || viper.GetString("log-file") != "" {
		f.Close()
	}
}

func logConfigKeysValues() {
	if viper.GetBool("debug") {
		b, err := json.MarshalIndent(viper.AllSettings(), "", "  ")
		if err != nil {
			logger.Printf("could not marshal viper settings: %v", err)
		} else {
			logger.Printf("set flags/config:\n%s\n", string(b))
		}
		keys := viper.AllKeys()
		sort.Strings(keys)

		for _, k := range keys {
			if !viper.IsSet(k) {
				continue
			}
			v := viper.Get(k)
			logger.Printf("%s='%v'(%T)", k, v, v)
		}
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd *cobra.Command

func newRootCmd() *cobra.Command {
	rootCmd = &cobra.Command{
		Use:   "gnmic",
		Short: "run gnmi rpcs from the terminal (https://gnmic.kmrd.dev)",
		Annotations: map[string]string{
			"--encoding": "ENCODING",
			"--config":   "FILE",
			"--format":   "FORMAT",
			"--address":  "TARGET",
		},
		PersistentPreRunE: rootCmdPersistentPreRunE,
		PersistentPostRun: rootCmdPersistentPostRun,
	}
	initGlobalflags(rootCmd, cfg.Globals)
	rootCmd.AddCommand(newCapabilitiesCmd())
	rootCmd.AddCommand(newGetCmd())
	rootCmd.AddCommand(newListenCmd())
	rootCmd.AddCommand(newPathCmd())
	rootCmd.AddCommand(newPromptCmd())
	rootCmd.AddCommand(newSetCmd())
	rootCmd.AddCommand(newSubscribeCmd())
	versionCmd := newVersionCmd()
	versionCmd.AddCommand(newVersionUpgradeCmd())
	rootCmd.AddCommand(versionCmd)
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		//fmt.Println(err)
		os.Exit(1)
	}
	if promptMode {
		ExecutePrompt()
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initGlobalflags(cmd *cobra.Command, globals *config.GlobalFlags) {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/gnmic.yaml)")
	rootCmd.PersistentFlags().StringSliceVarP(&globals.Address, "address", "a", []string{}, "comma separated gnmi targets addresses")
	rootCmd.PersistentFlags().StringVarP(&globals.Username, "username", "u", "", "username")
	rootCmd.PersistentFlags().StringVarP(&globals.Password, "password", "p", "", "password")
	rootCmd.PersistentFlags().StringVarP(&globals.Port, "port", "", defaultGrpcPort, "gRPC port")
	rootCmd.PersistentFlags().StringVarP(&globals.Encoding, "encoding", "e", "json", fmt.Sprintf("one of %q. Case insensitive", encodingNames))
	rootCmd.PersistentFlags().BoolVarP(&globals.Insecure, "insecure", "", false, "insecure connection")
	rootCmd.PersistentFlags().StringVarP(&globals.TLSCa, "tls-ca", "", "", "tls certificate authority")
	rootCmd.PersistentFlags().StringVarP(&globals.TLSCert, "tls-cert", "", "", "tls certificate")
	rootCmd.PersistentFlags().StringVarP(&globals.TLSKey, "tls-key", "", "", "tls key")
	rootCmd.PersistentFlags().DurationVarP(&globals.Timeout, "timeout", "", 10*time.Second, "grpc timeout, valid formats: 10s, 1m30s, 1h")
	rootCmd.PersistentFlags().BoolVarP(&globals.Debug, "debug", "d", false, "debug mode")
	rootCmd.PersistentFlags().BoolVarP(&globals.SkipVerify, "skip-verify", "", false, "skip verify tls connection")
	rootCmd.PersistentFlags().BoolVarP(&globals.NoPrefix, "no-prefix", "", false, "do not add [ip:port] prefix to print output in case of multiple targets")
	rootCmd.PersistentFlags().BoolVarP(&globals.ProxyFromEnv, "proxy-from-env", "", false, "use proxy from environment")
	rootCmd.PersistentFlags().StringVarP(&globals.Format, "format", "", "", fmt.Sprintf("output format, one of: %q", formatNames))
	rootCmd.PersistentFlags().StringVarP(&globals.LogFile, "log-file", "", "", "log file path")
	rootCmd.PersistentFlags().BoolVarP(&globals.Log, "log", "", false, "show log messages in stderr")
	rootCmd.PersistentFlags().IntVarP(&globals.MaxMsgSize, "max-msg-size", "", msgSize, "max grpc msg size")
	rootCmd.PersistentFlags().StringVarP(&globals.PrometheusAddress, "prometheus-address", "", "", "prometheus server address")
	rootCmd.PersistentFlags().BoolVarP(&globals.PrintRequest, "print-request", "", false, "print request as well as the response(s)")
	rootCmd.PersistentFlags().DurationVarP(&globals.Retry, "retry", "", defaultRetryTimer, "retry timer for RPCs")
	rootCmd.PersistentFlags().StringVarP(&globals.TLSMinVersion, "tls-min-version", "", "", fmt.Sprintf("minimum TLS supported version, one of %q", tlsVersions))
	rootCmd.PersistentFlags().StringVarP(&globals.TLSMaxVersion, "tls-max-version", "", "", fmt.Sprintf("maximum TLS supported version, one of %q", tlsVersions))
	rootCmd.PersistentFlags().StringVarP(&globals.TLSVersion, "tls-version", "", "", fmt.Sprintf("set TLS version. Overwrites --tls-min-version and --tls-max-version, one of %q", tlsVersions))
	// to remove
	viper.BindPFlag("address", rootCmd.PersistentFlags().Lookup("address"))
	viper.BindPFlag("username", rootCmd.PersistentFlags().Lookup("username"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("encoding", rootCmd.PersistentFlags().Lookup("encoding"))
	viper.BindPFlag("insecure", rootCmd.PersistentFlags().Lookup("insecure"))
	viper.BindPFlag("tls-ca", rootCmd.PersistentFlags().Lookup("tls-ca"))
	viper.BindPFlag("tls-cert", rootCmd.PersistentFlags().Lookup("tls-cert"))
	viper.BindPFlag("tls-key", rootCmd.PersistentFlags().Lookup("tls-key"))
	viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("skip-verify", rootCmd.PersistentFlags().Lookup("skip-verify"))
	viper.BindPFlag("no-prefix", rootCmd.PersistentFlags().Lookup("no-prefix"))
	viper.BindPFlag("proxy-from-env", rootCmd.PersistentFlags().Lookup("proxy-from-env"))
	viper.BindPFlag("format", rootCmd.PersistentFlags().Lookup("format"))
	viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	viper.BindPFlag("log", rootCmd.PersistentFlags().Lookup("log"))
	viper.BindPFlag("max-msg-size", rootCmd.PersistentFlags().Lookup("max-msg-size"))
	viper.BindPFlag("prometheus-address", rootCmd.PersistentFlags().Lookup("prometheus-address"))
	viper.BindPFlag("print-request", rootCmd.PersistentFlags().Lookup("print-request"))
	viper.BindPFlag("retry", rootCmd.PersistentFlags().Lookup("retry"))
	viper.BindPFlag("tls-min-version", rootCmd.PersistentFlags().Lookup("tls-min-version"))
	viper.BindPFlag("tls-max-version", rootCmd.PersistentFlags().Lookup("tls-max-version"))
	viper.BindPFlag("tls-version", rootCmd.PersistentFlags().Lookup("tls-version"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".gnmic" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName("gnmic")
	}

	//viper.AutomaticEnv() // read in environment variables that match
	err := cfg.Load(cfgFile)
	if err != nil {
		fmt.Println("failed loading config: ", err)
	}
	fmt.Printf("globals: %+v\n", cfg.Globals)
	// If a config file is found, read it in.
	cfg.SetFlagsFromFile(rootCmd)
	postInitCommands(rootCmd.Commands())
}

func postInitCommands(commands []*cobra.Command) {
	for _, cmd := range commands {
		presetRequiredFlags(cmd)
		if cmd.HasSubCommands() {
			postInitCommands(cmd.Commands())
		}
	}
}

func presetRequiredFlags(cmd *cobra.Command) {
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		flagName := fmt.Sprintf("%s-%s", cmd.Name(), f.Name)
		value := viper.Get(flagName)
		if value != nil && viper.IsSet(flagName) && !f.Changed {
			var err error
			switch value := value.(type) {
			case string:
				err = cmd.LocalFlags().Set(f.Name, value)
			case []interface{}:
				ls := make([]string, len(value))
				for i := range value {
					ls[i] = value[i].(string)
				}
				err = cmd.LocalFlags().Set(f.Name, strings.Join(ls, ","))
			case []string:
				err = cmd.LocalFlags().Set(f.Name, strings.Join(value, ","))
			default:
				fmt.Printf("unexpected config value type, value=%v, type=%T\n", value, value)
			}
			if err != nil {
				fmt.Printf("failed setting flag '%s' from viper: %v\n", flagName, err)
			}
		}
	})
}

func loadCerts(tlscfg *tls.Config) error {
	tlsCert := viper.GetString("tls-cert")
	tlsKey := viper.GetString("tls-key")
	if tlsCert != "" && tlsKey != "" {
		certificate, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			return err
		}
		tlscfg.Certificates = []tls.Certificate{certificate}
		tlscfg.BuildNameToCertificate()
	}
	return nil
}
func loadCACerts(tlscfg *tls.Config) error {
	tlsCa := viper.GetString("tls-ca")
	certPool := x509.NewCertPool()
	if tlsCa != "" {
		caFile, err := ioutil.ReadFile(tlsCa)
		if err != nil {
			return err
		}
		if ok := certPool.AppendCertsFromPEM(caFile); !ok {
			return errors.New("failed to append certificate")
		}
		tlscfg.RootCAs = certPool
	}
	return nil
}
func printer(ctx context.Context, c chan string) {
	for {
		select {
		case m := <-c:
			fmt.Println(m)
		case <-ctx.Done():
			return
		}
	}
}
func gather(ctx context.Context, c chan string, ls *[]string) {
	for {
		select {
		case m := <-c:
			*ls = append(*ls, m)
		case <-ctx.Done():
			return
		}
	}
}

type myWriteCloser struct {
	io.Writer
}

func (myWriteCloser) Close() error {
	return nil
}

func indent(prefix, s string) string {
	if prefix == "" {
		return s
	}
	prefix = "\n" + strings.TrimRight(prefix, "\n")
	lines := strings.Split(s, "\n")
	return strings.TrimLeft(fmt.Sprintf("%s%s", prefix, strings.Join(lines, prefix)), "\n")
}

func filterModels(ctx context.Context, coll *collector.Collector, tName string, modelsNames []string) (map[string]*gnmi.ModelData, []string, error) {
	supModels, err := coll.GetModels(ctx, tName)
	if err != nil {
		return nil, nil, err
	}
	unsupportedModels := make([]string, 0)
	supportedModels := make(map[string]*gnmi.ModelData)
	var found bool
	for _, m := range modelsNames {
		found = false
		for _, tModel := range supModels {
			if m == tModel.Name {
				supportedModels[m] = tModel
				found = true
				break
			}
		}
		if !found {
			unsupportedModels = append(unsupportedModels, m)
		}
	}
	return supportedModels, unsupportedModels, nil
}
func setupCloseHandler(cancelFn context.CancelFunc) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-c
		fmt.Printf("\nreceived signal '%s'. terminating...\n", sig.String())
		cancelFn()
		os.Exit(0)
	}()
}

func numTargets() int {
	addressesLength := len(viper.GetStringSlice("address"))
	if addressesLength > 0 {
		return addressesLength
	}
	targets := viper.Get("targets")
	switch targets := targets.(type) {
	case string:
		return len(strings.Split(targets, " "))
	case map[string]interface{}:
		return len(targets)
	}
	return 0
}

func createCollectorDialOpts() []grpc.DialOption {
	opts := []grpc.DialOption{}
	opts = append(opts, grpc.WithBlock())
	if viper.GetInt("max-msg-size") > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(viper.GetInt("max-msg-size"))))
	}
	if !viper.GetBool("proxy-from-env") {
		opts = append(opts, grpc.WithNoProxy())
	}
	return opts
}

func printMsg(address string, msg proto.Message) error {
	printPrefix := ""
	if numTargets() > 1 && !viper.GetBool("no-prefix") {
		printPrefix = fmt.Sprintf("[%s] ", address)
	}

	switch msg := msg.ProtoReflect().Interface().(type) {
	case *gnmi.CapabilityResponse:
		if len(viper.GetString("format")) == 0 {
			printCapResponse(printPrefix, msg)
			return nil
		}
	}
	mo := formatters.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
		Format:    viper.GetString("format"),
	}
	b, err := mo.Marshal(msg, map[string]string{"address": address})
	if err != nil {
		return err
	}
	sb := strings.Builder{}
	sb.Write(b)
	fmt.Printf("%s\n", indent(printPrefix, sb.String()))
	return nil
}

func printCapResponse(printPrefix string, msg *gnmi.CapabilityResponse) {
	sb := strings.Builder{}
	sb.WriteString(printPrefix)
	sb.WriteString("gNMI version: ")
	sb.WriteString(msg.GNMIVersion)
	sb.WriteString("\n")
	if viper.GetBool("version") {
		return
	}
	sb.WriteString(printPrefix)
	sb.WriteString("supported models:\n")
	for _, sm := range msg.SupportedModels {
		sb.WriteString(printPrefix)
		sb.WriteString("  - ")
		sb.WriteString(sm.GetName())
		sb.WriteString(", ")
		sb.WriteString(sm.GetOrganization())
		sb.WriteString(", ")
		sb.WriteString(sm.GetVersion())
		sb.WriteString("\n")
	}
	sb.WriteString(printPrefix)
	sb.WriteString("supported encodings:\n")
	for _, se := range msg.SupportedEncodings {
		sb.WriteString(printPrefix)
		sb.WriteString("  - ")
		sb.WriteString(se.String())
		sb.WriteString("\n")
	}
	fmt.Printf("%s\n", indent(printPrefix, sb.String()))
}
