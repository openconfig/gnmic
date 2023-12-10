// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"github.com/openconfig/gnmic/pkg/app"
	"github.com/openconfig/gnmic/pkg/cmd/capabilities"
	"github.com/openconfig/gnmic/pkg/cmd/diff"
	"github.com/openconfig/gnmic/pkg/cmd/generate"
	"github.com/openconfig/gnmic/pkg/cmd/get"
	"github.com/openconfig/gnmic/pkg/cmd/getset"
	"github.com/openconfig/gnmic/pkg/cmd/listener"
	"github.com/openconfig/gnmic/pkg/cmd/path"
	"github.com/openconfig/gnmic/pkg/cmd/set"
	"github.com/openconfig/gnmic/pkg/cmd/subscribe"
	"github.com/openconfig/gnmic/pkg/cmd/version"
	"github.com/spf13/cobra"
)

var encodings = [][2]string{
	{"json", "JSON encoded string (RFC7159)"},
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

var gApp = app.New()

func newRootCmd() *cobra.Command {
	gApp.RootCmd = &cobra.Command{
		Use:   "gnmic",
		Short: "run gnmi rpcs from the terminal (https://gnmic.openconfig.net)",
		Annotations: map[string]string{
			"--encoding": "ENCODING",
			"--config":   "FILE",
			"--format":   "FORMAT",
			"--address":  "TARGET",
		},
		PersistentPreRunE: gApp.PreRunE,
	}
	gApp.InitGlobalFlags()
	gApp.RootCmd.AddCommand(newCompletionCmd(gApp))
	gApp.RootCmd.AddCommand(newPromptCmd())

	// Subcommands
	gApp.RootCmd.AddCommand(capabilities.New(gApp))
	gApp.RootCmd.AddCommand(get.New(gApp))
	gApp.RootCmd.AddCommand(getset.New(gApp))
	gApp.RootCmd.AddCommand(listener.New(gApp))
	gApp.RootCmd.AddCommand(path.New(gApp))
	gApp.RootCmd.AddCommand(diff.New(gApp))
	gApp.RootCmd.AddCommand(generate.New(gApp))
	gApp.RootCmd.AddCommand(set.New(gApp))
	gApp.RootCmd.AddCommand(subscribe.New(gApp))
	gApp.RootCmd.AddCommand(version.New(gApp))
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

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	err := gApp.Config.Load(gApp.Context())
	if err == nil {
		return
	}
	if _, ok := err.(*fs.PathError); !ok {
		fmt.Fprintf(os.Stderr, "failed loading config file: %v\n", err)
	}
}

func setupCloseHandler(cancelFn context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-c
		fmt.Printf("\nreceived signal '%s'. terminating...\n", sig.String())
		cancelFn()
		os.Exit(0)
	}()
}
