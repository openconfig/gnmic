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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/karimra/gnmic/collector"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
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

var encodings = [][2]string{
	{"json", "JSON encodded string (RFC7159)"},
	{"bytes", "byte sequence whose semantics is opaque to the protocol"},
	{"proto", "serialised protobuf message using protobuf.Any"},
	{"ascii", "ASCII encoded string representing text formatted according to a target-defined convention"},
	{"json_ietf", "JSON_IETF encoded string (RFC7951)"},
}
var formats = [][2]string{
	{"json", "similar to protojson but with xpath style paths and decoded timestamps"},
	{"protojson", "protocol buffer messages in JSON format"},
	{"prototext", "protocol buffer messages in textproto format"},
	{"event", "protocol buffer messages as a timestamped list of tags and values"},
	{"proto", "protocol buffer messages in binary wire format"},
}
var cfgFile string
var f io.WriteCloser
var logger *log.Logger

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
	return nil
}

func rootCmdPersistentPostRun(cmd *cobra.Command, args []string) {
	if !viper.GetBool("log") || viper.GetString("log-file") != "" {
		f.Close()
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gnmic",
	Short: "run gnmi rpcs from the terminal (https://gnmic.kmrd.dev)",
	Annotations: map[string]string{
		"--encoding": "ENCODING",
		"--config":   "FILE",
		"--format":   "FORMAT",
	},
	PersistentPreRunE: rootCmdPersistentPreRunE,
	PersistentPostRun:  rootCmdPersistentPostRun,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		//fmt.Println(err)
		os.Exit(1)
	}
	if promptMode {
		ExecutePrompt()
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/gnmic.yaml)")
	rootCmd.PersistentFlags().StringSliceP("address", "a", []string{}, "comma separated gnmi targets addresses")
	rootCmd.PersistentFlags().StringP("username", "u", "", "username")
	rootCmd.PersistentFlags().StringP("password", "p", "", "password")
	rootCmd.PersistentFlags().StringP("port", "", defaultGrpcPort, "gRPC port")
	rootCmd.PersistentFlags().StringP("encoding", "e", "json", fmt.Sprintf("one of %+v. Case insensitive", encodings))
	rootCmd.PersistentFlags().BoolP("insecure", "", false, "insecure connection")
	rootCmd.PersistentFlags().StringP("tls-ca", "", "", "tls certificate authority")
	rootCmd.PersistentFlags().StringP("tls-cert", "", "", "tls certificate")
	rootCmd.PersistentFlags().StringP("tls-key", "", "", "tls key")
	rootCmd.PersistentFlags().DurationP("timeout", "", 10*time.Second, "grpc timeout, valid formats: 10s, 1m30s, 1h")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "debug mode")
	rootCmd.PersistentFlags().BoolP("skip-verify", "", false, "skip verify tls connection")
	rootCmd.PersistentFlags().BoolP("no-prefix", "", false, "do not add [ip:port] prefix to print output in case of multiple targets")
	rootCmd.PersistentFlags().BoolP("proxy-from-env", "", false, "use proxy from environment")
	rootCmd.PersistentFlags().StringP("format", "", "", "output format, one of: [protojson, prototext, json, event]")
	rootCmd.PersistentFlags().StringP("log-file", "", "", "log file path")
	rootCmd.PersistentFlags().BoolP("log", "", false, "show log messages in stderr")
	rootCmd.PersistentFlags().IntP("max-msg-size", "", msgSize, "max grpc msg size")
	rootCmd.PersistentFlags().StringP("prometheus-address", "", "", "prometheus server address")
	rootCmd.PersistentFlags().BoolP("print-request", "", false, "print request as well as the response(s)")
	rootCmd.PersistentFlags().DurationP("retry", "", defaultRetryTimer, "retry timer for RPCs")
	//
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

	// If a config file is found, read it in.
	viper.ReadInConfig()
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

func readUsername() (string, error) {
	var username string
	fmt.Print("username: ")
	_, err := fmt.Scan(&username)
	if err != nil {
		return "", err
	}
	return username, nil
}
func readPassword() (string, error) {
	fmt.Print("password: ")
	pass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(pass), nil
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

func filterModels(ctx context.Context, t *collector.Target, modelsNames []string) (map[string]*gnmi.ModelData, []string, error) {
	capResp, err := t.Capabilities(ctx)
	if err != nil {
		return nil, nil, err
	}
	unsupportedModels := make([]string, 0)
	supportedModels := make(map[string]*gnmi.ModelData)
	var found bool
	for _, m := range modelsNames {
		found = false
		for _, tModel := range capResp.SupportedModels {
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

func createTargets() (map[string]*collector.TargetConfig, error) {
	addresses := viper.GetStringSlice("address")
	targets := make(map[string]*collector.TargetConfig)
	defGrpcPort := viper.GetString("port")
	// case address is defined in config file
	if len(addresses) > 0 {
		if viper.GetString("username") == "" {
			defUsername, err := readUsername()
			if err != nil {
				return nil, err
			}
			viper.Set("username", defUsername)
		}
		if viper.GetString("password") == "" {
			defPassword, err := readPassword()
			if err != nil {
				return nil, err
			}
			viper.Set("password", defPassword)
		}
		for _, addr := range addresses {
			tc := new(collector.TargetConfig)
			if !strings.HasPrefix(addr, "unix://") {
				_, _, err := net.SplitHostPort(addr)
				if err != nil {
					if strings.Contains(err.Error(), "missing port in address") {
						addr = net.JoinHostPort(addr, defGrpcPort)
					} else {
						logger.Printf("error parsing address '%s': %v", addr, err)
						return nil, fmt.Errorf("error parsing address '%s': %v", addr, err)
					}
				}
			}
			tc.Address = addr
			setTargetConfigDefaults(tc)
			targets[tc.Name] = tc
		}
		return targets, nil
	}
	// case targets is defined in config file
	targetsInt := viper.Get("targets")
	targetsMap := make(map[string]interface{})
	switch targetsInt := targetsInt.(type) {
	case string:
		for _, addr := range strings.Split(targetsInt, " ") {
			targetsMap[addr] = nil
		}
	case map[string]interface{}:
		targetsMap = targetsInt
	default:
		return nil, fmt.Errorf("unexpected targets format, got: %T", targetsInt)
	}
	if len(targetsMap) == 0 {
		return nil, fmt.Errorf("no targets found")
	}
	for addr, t := range targetsMap {
		if !strings.HasPrefix(addr, "unix://") {
			_, _, err := net.SplitHostPort(addr)
			if err != nil {
				if strings.Contains(err.Error(), "missing port in address") {
					addr = net.JoinHostPort(addr, defGrpcPort)
				} else {
					logger.Printf("error parsing address '%s': %v", addr, err)
					return nil, fmt.Errorf("error parsing address '%s': %v", addr, err)
				}
			}
		}
		tc := new(collector.TargetConfig)
		switch t := t.(type) {
		case map[string]interface{}:
			decoder, err := mapstructure.NewDecoder(
				&mapstructure.DecoderConfig{
					DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
					Result:     tc,
				},
			)
			if err != nil {
				return nil, err
			}
			err = decoder.Decode(t)
			if err != nil {
				return nil, err
			}
		case nil:
		default:
			return nil, fmt.Errorf("unexpected targets format, got a %T", t)
		}
		tc.Address = addr
		setTargetConfigDefaults(tc)
		if viper.GetBool("debug") {
			logger.Printf("read target config: %s", tc)
		}
		targets[tc.Name] = tc
	}
	subNames := viper.GetStringSlice("subscribe-name")
	if len(subNames) == 0 {
		return targets, nil
	}
	for n := range targets {
		targets[n].Subscriptions = subNames
	}
	return targets, nil
}

func setTargetConfigDefaults(tc *collector.TargetConfig) {
	if tc.Name == "" {
		tc.Name = tc.Address
	}
	if tc.Username == nil {
		s := viper.GetString("username")
		tc.Username = &s
	}
	if tc.Password == nil {
		s := viper.GetString("password")
		tc.Password = &s
	}
	if tc.Timeout == 0 {
		tc.Timeout = viper.GetDuration("timeout")
	}
	if tc.Insecure == nil {
		b := viper.GetBool("insecure")
		tc.Insecure = &b
	}
	if tc.SkipVerify == nil {
		b := viper.GetBool("skip-verify")
		tc.SkipVerify = &b
	}
	if tc.Insecure != nil && !*tc.Insecure {
		if tc.TLSCA == nil {
			s := viper.GetString("tls-ca")
			if s != "" {
				tc.TLSCA = &s
			}
		}
		if tc.TLSCert == nil {
			s := viper.GetString("tls-cert")
			tc.TLSCert = &s
		}
		if tc.TLSKey == nil {
			s := viper.GetString("tls-key")
			tc.TLSKey = &s
		}
	}
	if tc.RetryTimer == 0 {
		tc.RetryTimer = viper.GetDuration("retry")
	}
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
	mo := collector.MarshalOptions{
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
