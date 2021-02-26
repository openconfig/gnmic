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
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/karimra/gnmic/app"
	"github.com/karimra/gnmic/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	defaultGrpcPort = "57400"
	msgSize         = 512 * 1024 * 1024
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

// rootCmd represents the base command when called without any subcommands
// var rootCmd *cobra.Command

var gApp = app.New()

func newRootCmd() *cobra.Command {
	gApp.RootCmd = &cobra.Command{
		Use:   "gnmic",
		Short: "run gnmi rpcs from the terminal (https://gnmic.kmrd.dev)",
		Annotations: map[string]string{
			"--encoding": "ENCODING",
			"--config":   "FILE",
			"--format":   "FORMAT",
			"--address":  "TARGET",
		},
		PersistentPreRunE: gApp.PreRun,
	}
	initGlobalflags(gApp.RootCmd, gApp.Config)
	gApp.RootCmd.AddCommand(newCapabilitiesCmd())
	gApp.RootCmd.AddCommand(newGetCmd())
	gApp.RootCmd.AddCommand(newListenCmd())
	gApp.RootCmd.AddCommand(newPathCmd())
	gApp.RootCmd.AddCommand(newPromptCmd())
	gApp.RootCmd.AddCommand(newSetCmd())
	gApp.RootCmd.AddCommand(newSubscribeCmd())
	versionCmd := newVersionCmd()
	versionCmd.AddCommand(newVersionUpgradeCmd())
	gApp.RootCmd.AddCommand(versionCmd)
	return gApp.RootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	setupCloseHandler(gApp.Cfn)
	if err := newRootCmd().Execute(); err != nil {
		//fmt.Println(err)
		os.Exit(1)
	}
	if gApp.PromptMode {
		ExecutePrompt()
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initGlobalflags(cmd *cobra.Command, globals *config.Config) {
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
	cmd.PersistentFlags().StringVarP(&globals.InstanceName, "instance-name", "", "", "gnmic instance name")
	cmd.PersistentFlags().StringVarP(&globals.API, "api", "", "", "gnmic api address")

	cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		gApp.Config.FileConfig.BindPFlag(flag.Name, flag)
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	err := gApp.Config.Load(cfgFile)
	if err == nil {
		return
	}
	if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
		fmt.Fprintf(os.Stderr, "failed loading config file: %v\n", err)
	}
}

func loadCerts(tlscfg *tls.Config) error {
	if gApp.Config.TLSCert != "" && gApp.Config.TLSKey != "" {
		certificate, err := tls.LoadX509KeyPair(gApp.Config.TLSCert, gApp.Config.TLSKey)
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
	if gApp.Config.TLSCa != "" {
		caFile, err := ioutil.ReadFile(gApp.Config.TLSCa)
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
