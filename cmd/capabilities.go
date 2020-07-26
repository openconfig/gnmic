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
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/karimra/gnmic/collector"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/spf13/viper"

	"github.com/spf13/cobra"
)

var printVersion bool

// capabilitiesCmd represents the capabilities command
var capabilitiesCmd = &cobra.Command{
	Use:     "capabilities",
	Aliases: []string{"c", "cap"},
	Short:   "query targets gnmi capabilities",

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		setupCloseHandler(cancel)
		targets, err := createTargets()
		if err != nil {
			return err
		}
		wg := new(sync.WaitGroup)
		wg.Add(len(targets))
		lock := new(sync.Mutex)
		for _, tc := range targets {
			go reqCapability(ctx, collector.NewTarget(tc), wg, lock)
		}
		wg.Wait()
		return nil
	},
}

func reqCapability(ctx context.Context, target *collector.Target, wg *sync.WaitGroup, m *sync.Mutex) {
	defer wg.Done()
	opts := createCollectorDialOpts()
	if err := target.CreateGNMIClient(ctx, opts...); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Printf("failed to create a gRPC client for target '%s', timeout (%s) reached", target.Config.Name, target.Config.Timeout)
			return
		}
		logger.Printf("failed to create a gRPC client for target '%s' : %v", target.Config.Name, err)
		return
	}
	ext := make([]*gnmi_ext.Extension, 0) //
	logger.Printf("sending gNMI CapabilityRequest: gnmi_ext.Extension='%v' to %s", ext, target.Config.Address)
	response, err := target.Capabilities(ctx)
	if err != nil {
		logger.Printf("error sending capabilities request: %v", err)
		return
	}
	m.Lock()
	printCapResponse(response, target.Config.Address)
	m.Unlock()
}

func printCapResponse(r *gnmi.CapabilityResponse, address string) {
	printPrefix := ""
	addresses := viper.GetStringSlice("address")
	if len(addresses) > 1 && !viper.GetBool("no-prefix") {
		printPrefix = fmt.Sprintf("[%s] ", address)
	}
	format := viper.GetString("format")
	if len(format) > 0 {
		mo := collector.MarshalOptions{
			Multiline: true,
			Indent:    "  ",
			Format:    viper.GetString("format"),
		}
		b, err := mo.Marshal(r, nil)
		if err != nil {
			logger.Printf("error marshaling msg: %v", err)
			if !viper.GetBool("log") {
				fmt.Printf("error marshaling msg: %v\n", err)
			}
		}
		fmt.Printf("%s\n", indent(printPrefix, string(b)))
		return
	} else {
		sb := strings.Builder{}
		sb.WriteString(printPrefix)
		sb.WriteString("gNMI version: ")
		sb.WriteString(r.GNMIVersion)
		sb.WriteString("\n")
		if viper.GetBool("version") {
			return
		}
		sb.WriteString(printPrefix)
		sb.WriteString("supported models:\n")
		for _, sm := range r.SupportedModels {
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
		for _, se := range r.SupportedEncodings {
			sb.WriteString(printPrefix)
			sb.WriteString("  - ")
			sb.WriteString(se.String())
			sb.WriteString("\n")
		}
		fmt.Printf("%s\n", indent(printPrefix, sb.String()))
	}
}

func init() {
	rootCmd.AddCommand(capabilitiesCmd)
	capabilitiesCmd.Flags().BoolVarP(&printVersion, "version", "", false, "show gnmi version only")
	viper.BindPFlag("capabilities-version", capabilitiesCmd.LocalFlags().Lookup("version"))
}
