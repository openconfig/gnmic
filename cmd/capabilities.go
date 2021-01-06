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
	"fmt"

	"github.com/karimra/gnmic/collector"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// capabilitiesCmd represents the capabilities command
func newCapabilitiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "capabilities",
		Aliases: []string{"cap"},
		Short:   "query targets gnmi capabilities",
		PreRun: func(cmd *cobra.Command, args []string) {
			cli.config.SetLocalFlagsFromFile(cmd)
		},
		RunE: cli.capabilitiesRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initCapabilitiesFlags(cmd)
		},
		SilenceUsage: true,
	}
	initCapabilitiesFlags(cmd)
	return cmd
}

func (c *CLI) reqCapabilities(ctx context.Context, tName string) {
	defer c.wg.Done()
	ext := make([]*gnmi_ext.Extension, 0) //
	if c.config.Globals.PrintRequest {
		err := c.printMsg(tName, "Capabilities Request:", &gnmi.CapabilityRequest{
			Extension: ext,
		})
		if err != nil {
			cli.logger.Printf("%v", err)
			if !c.config.Globals.Log {
				fmt.Printf("target %s: %v\n", tName, err)
			}
		}
	}

	c.logger.Printf("sending gNMI CapabilityRequest: gnmi_ext.Extension='%v' to %s", ext, tName)
	response, err := c.collector.Capabilities(ctx, tName, ext...)
	if err != nil {
		c.logger.Printf("error sending capabilities request: %v", err)
		return
	}

	err = c.printMsg(tName, "Capabilities Response:", response)
	if err != nil {
		cli.logger.Printf("target %s: %v", tName, err)
	}
}

func initCapabilitiesFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&cli.config.LocalFlags.CapabilitiesVersion, "version", "", false, "show gnmi version only")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		cli.config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func (c *CLI) capabilitiesRunE(cmd *cobra.Command, args []string) error {
	if c.config.Globals.Format == "event" {
		return fmt.Errorf("format event not supported for Capabilities RPC")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupCloseHandler(cancel)
	targetsConfig, err := c.config.GetTargets()
	if err != nil {
		c.logger.Printf("failed getting targets config: %v", err)
		return fmt.Errorf("failed getting targets config: %v", err)
	}
	if c.collector == nil {
		cfg := &collector.Config{
			Debug:               c.config.Globals.Debug,
			Format:              c.config.Globals.Format,
			TargetReceiveBuffer: c.config.Globals.TargetBufferSize,
			RetryTimer:          c.config.Globals.Retry,
		}

		c.collector = collector.NewCollector(cfg, targetsConfig,
			collector.WithDialOptions(createCollectorDialOpts()),
			collector.WithLogger(c.logger),
		)
	} else {
		// prompt mode
		for _, tc := range targetsConfig {
			c.collector.AddTarget(tc)
		}
	}
	c.wg.Add(len(cli.collector.Targets))
	for tName := range cli.collector.Targets {
		go c.reqCapabilities(ctx, tName)
	}
	c.wg.Wait()
	return nil
}
