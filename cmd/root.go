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
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/formatters"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
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

type CLI struct {
	m         *sync.Mutex
	config    *config.Config
	collector *collector.Collector
	logger    *log.Logger

	promptMode    bool
	promptHistory []string
	schemaTree    *yang.Entry

	wg        *sync.WaitGroup
	printLock *sync.Mutex
}

var cli = &CLI{
	m:             new(sync.Mutex),
	config:        config.New(),
	logger:        log.New(ioutil.Discard, "", log.LstdFlags),
	promptHistory: make([]string, 0, 128),
	schemaTree: &yang.Entry{
		Dir: make(map[string]*yang.Entry),
	},

	wg:        new(sync.WaitGroup),
	printLock: new(sync.Mutex),
}

var cfgFile string

func rootCmdPersistentPreRunE(cmd *cobra.Command, args []string) error {
	cli.config.SetLogger()
	cli.config.SetPersistantFlagsFromFile(rootCmd)
	cli.config.Globals.Address = config.SanitizeArrayFlagValue(cli.config.Globals.Address)
	cli.logger = log.New(ioutil.Discard, "gnmic ", log.LstdFlags|log.Lmicroseconds)
	if cli.config.Globals.LogFile != "" {
		f, err := os.OpenFile(cli.config.Globals.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("error opening log file: %v", err)
		}
		cli.logger.SetOutput(f)
	} else {
		if cli.config.Globals.Debug {
			cli.config.Globals.Log = true
		}
		if cli.config.Globals.Log {
			cli.logger.SetOutput(os.Stderr)
		}
	}
	if cli.config.Globals.Debug {
		cli.logger.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Llongfile)
	}

	if cli.config.Globals.Debug {
		grpclog.SetLogger(cli.logger) //lint:ignore SA1019 see https://github.com/karimra/gnmic/issues/59
		cli.logger.Printf("version=%s, commit=%s, date=%s, gitURL=%s, docs=https://gnmic.kmrd.dev", version, commit, date, gitURL)
	}
	cfgFile := cli.config.FileConfig.ConfigFileUsed()
	if len(cfgFile) != 0 {
		cli.logger.Printf("using config file %s", cfgFile)
		b, err := ioutil.ReadFile(cfgFile)
		if err != nil {
			if cmd.Flag("config").Changed {
				return err
			}
			cli.logger.Printf("failed reading config file: %v", err)
		}
		if cli.config.Globals.Debug {
			cli.logger.Printf("config file:\n%s", string(b))
		}
	}
	logConfigKeysValues()
	return nil
}

func logConfigKeysValues() {
	if cli.config.Globals.Debug {
		b, err := json.MarshalIndent(cli.config.FileConfig.AllSettings(), "", "  ")
		if err != nil {
			cli.logger.Printf("could not marshal settings: %v", err)
		} else {
			cli.logger.Printf("set flags/config:\n%s\n", string(b))
		}
		keys := cli.config.FileConfig.AllKeys()
		sort.Strings(keys)

		for _, k := range keys {
			if !cli.config.FileConfig.IsSet(k) {
				continue
			}
			v := cli.config.FileConfig.Get(k)
			cli.logger.Printf("%s='%v'(%T)", k, v, v)
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
	}
	initGlobalflags(rootCmd, cli.config.Globals)
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
	if cli.promptMode {
		ExecutePrompt()
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initGlobalflags(cmd *cobra.Command, globals *config.GlobalFlags) {
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/gnmic.yaml)")
	cmd.PersistentFlags().StringSliceVarP(&globals.Address, "address", "a", []string{}, "comma separated gnmi targets addresses")
	cmd.PersistentFlags().StringVarP(&globals.Username, "username", "u", "", "username")
	cmd.PersistentFlags().StringVarP(&globals.Password, "password", "p", "", "password")
	cmd.PersistentFlags().StringVarP(&globals.Port, "port", "", defaultGrpcPort, "gRPC port")
	cmd.PersistentFlags().StringVarP(&globals.Encoding, "encoding", "e", "json", fmt.Sprintf("one of %q. Case insensitive", encodingNames))
	cmd.PersistentFlags().BoolVarP(&globals.Insecure, "insecure", "", false, "insecure connection")
	cmd.PersistentFlags().StringVarP(&globals.TLSCa, "tls-ca", "", "", "tls certificate authority")
	cmd.PersistentFlags().StringVarP(&globals.TLSCert, "tls-cert", "", "", "tls certificate")
	cmd.PersistentFlags().StringVarP(&globals.TLSKey, "tls-key", "", "", "tls key")
	cmd.PersistentFlags().DurationVarP(&globals.Timeout, "timeout", "", 10*time.Second, "grpc timeout, valid formats: 10s, 1m30s, 1h")
	cmd.PersistentFlags().BoolVarP(&globals.Debug, "debug", "d", false, "debug mode")
	cmd.PersistentFlags().BoolVarP(&globals.SkipVerify, "skip-verify", "", false, "skip verify tls connection")
	cmd.PersistentFlags().BoolVarP(&globals.NoPrefix, "no-prefix", "", false, "do not add [ip:port] prefix to print output in case of multiple targets")
	cmd.PersistentFlags().BoolVarP(&globals.ProxyFromEnv, "proxy-from-env", "", false, "use proxy from environment")
	cmd.PersistentFlags().StringVarP(&globals.Format, "format", "", "", fmt.Sprintf("output format, one of: %q", formatNames))
	cmd.PersistentFlags().StringVarP(&globals.LogFile, "log-file", "", "", "log file path")
	cmd.PersistentFlags().BoolVarP(&globals.Log, "log", "", false, "show log messages in stderr")
	cmd.PersistentFlags().IntVarP(&globals.MaxMsgSize, "max-msg-size", "", msgSize, "max grpc msg size")
	cmd.PersistentFlags().StringVarP(&globals.PrometheusAddress, "prometheus-address", "", "", "prometheus server address")
	cmd.PersistentFlags().BoolVarP(&globals.PrintRequest, "print-request", "", false, "print request as well as the response(s)")
	cmd.PersistentFlags().DurationVarP(&globals.Retry, "retry", "", defaultRetryTimer, "retry timer for RPCs")
	cmd.PersistentFlags().StringVarP(&globals.TLSMinVersion, "tls-min-version", "", "", fmt.Sprintf("minimum TLS supported version, one of %q", tlsVersions))
	cmd.PersistentFlags().StringVarP(&globals.TLSMaxVersion, "tls-max-version", "", "", fmt.Sprintf("maximum TLS supported version, one of %q", tlsVersions))
	cmd.PersistentFlags().StringVarP(&globals.TLSVersion, "tls-version", "", "", fmt.Sprintf("set TLS version. Overwrites --tls-min-version and --tls-max-version, one of %q", tlsVersions))

	cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		cli.config.FileConfig.BindPFlag(flag.Name, flag)
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	err := cli.config.Load(cfgFile)
	if err == nil {
		return
	}
	if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
		fmt.Printf("failed loading config file: %v\n", err)
	}
}

func loadCerts(tlscfg *tls.Config) error {
	if cli.config.Globals.TLSCert != "" && cli.config.Globals.TLSKey != "" {
		certificate, err := tls.LoadX509KeyPair(cli.config.Globals.TLSCert, cli.config.Globals.TLSKey)
		if err != nil {
			return err
		}
		tlscfg.Certificates = []tls.Certificate{certificate}
		tlscfg.BuildNameToCertificate()
	}
	return nil
}

func loadCACerts(tlscfg *tls.Config) error {
	certPool := x509.NewCertPool()
	if cli.config.Globals.TLSCa != "" {
		caFile, err := ioutil.ReadFile(cli.config.Globals.TLSCa)
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

func indent(prefix, s string) string {
	if prefix == "" {
		return s
	}
	prefix = "\n" + strings.TrimRight(prefix, "\n")
	lines := strings.Split(s, "\n")
	return strings.TrimLeft(fmt.Sprintf("%s%s", prefix, strings.Join(lines, prefix)), "\n")
}

func filterModels(ctx context.Context, coll *collector.Collector, tName string, modelsNames []string) (map[string]*gnmi.ModelData, []string, error) {
	supModels, err := cli.collector.GetModels(ctx, tName)
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

func createCollectorDialOpts() []grpc.DialOption {
	opts := []grpc.DialOption{}
	opts = append(opts, grpc.WithBlock())
	if cli.config.Globals.MaxMsgSize > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(cli.config.Globals.MaxMsgSize)))
	}
	if !cli.config.Globals.ProxyFromEnv {
		opts = append(opts, grpc.WithNoProxy())
	}
	return opts
}

func (c *CLI) printMsg(address string, msgName string, msg proto.Message) error {
	c.printLock.Lock()
	defer c.printLock.Unlock()
	fmt.Fprint(os.Stderr, msgName)
	fmt.Fprintln(os.Stderr, "")
	printPrefix := ""
	if len(cli.config.TargetsList()) > 1 && !cli.config.Globals.NoPrefix {
		printPrefix = fmt.Sprintf("[%s] ", address)
	}

	switch msg := msg.ProtoReflect().Interface().(type) {
	case *gnmi.CapabilityResponse:
		if len(cli.config.Globals.Format) == 0 {
			printCapResponse(printPrefix, msg)
			return nil
		}
	}
	mo := formatters.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
		Format:    cli.config.Globals.Format,
	}
	b, err := mo.Marshal(msg, map[string]string{"address": address})
	if err != nil {
		cli.logger.Printf("error marshaling capabilities request: %v", err)
		if !cli.config.Globals.Log {
			fmt.Printf("error marshaling capabilities request: %v", err)
		}
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
	if cli.config.LocalFlags.CapabilitiesVersion {
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

func (c *CLI) watchConfig() {
	c.logger.Printf("watching config...")
	c.config.FileConfig.OnConfigChange(c.loadTargets)
	c.config.FileConfig.WatchConfig()
}

func (c *CLI) loadTargets(e fsnotify.Event) {
	c.logger.Printf("got config change notification: %v", e)
	c.m.Lock()
	defer c.m.Unlock()
	switch e.Op {
	case fsnotify.Write, fsnotify.Create:
		newTargets, err := c.config.GetTargets()
		if err != nil && !errors.Is(err, config.ErrNoTargetsFound) {
			c.logger.Printf("failed getting targets from new config: %v", err)
			return
		}
		currentTargets := c.collector.Targets
		// delete targets
		for n := range currentTargets {
			if _, ok := newTargets[n]; !ok {
				err = c.collector.DeleteTarget(n)
				if err != nil {
					c.logger.Printf("failed to delete target %q: %v", n, err)
				}
			}
		}
		// add targets
		for n, tc := range newTargets {
			if _, ok := currentTargets[n]; !ok {
				err = c.collector.AddTarget(tc)
				if err != nil {
					c.logger.Printf("failed adding target %q: %v", n, err)
					continue
				}
				c.wg.Add(1)
				go c.collector.InitTarget(gctx, n)
			}
		}
	}
}
