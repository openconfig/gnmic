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
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//var paths []string
var dataType = [][2]string{
	{"all", "all config/state/operational data"},
	{"config", "data that the target considers to be read/write"},
	{"state", "read-only data on the target"},
	{"operational", "read-only data on the target that is related to software processes operating on the device, or external interactions of the device"},
}

// getCmd represents the get command
func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "run gnmi get on targets",
		Annotations: map[string]string{
			"--path":   "XPATH",
			"--prefix": "PREFIX",
			"--model":  "MODEL",
			"--type":   "STORE",
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			cli.config.SetLocalFlagsFromFile(cmd)
			cli.config.LocalFlags.GetPath = sanitizeArrayFlagValue(cli.config.LocalFlags.GetPath)
			cli.config.LocalFlags.GetModel = sanitizeArrayFlagValue(cli.config.LocalFlags.GetModel)
		},
		RunE: cli.getRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initGetFlags(cmd)
		},
		SilenceUsage: true,
	}
	initGetFlags(cmd)
	return cmd
}

func (c *CLI) getRequest(ctx context.Context, tName string, req *gnmi.GetRequest) {
	defer c.wg.Done()
	xreq := req
	if len(c.config.LocalFlags.GetModel) > 0 {
		spModels, unspModels, err := filterModels(ctx, c.collector, tName, c.config.LocalFlags.GetModel)
		if err != nil {
			c.logger.Printf("failed getting supported models from '%s': %v", tName, err)
			return
		}
		if len(unspModels) > 0 {
			c.logger.Printf("found unsupported models for target '%s': %+v", tName, unspModels)
		}
		for _, m := range spModels {
			xreq.UseModels = append(xreq.UseModels, m)
		}
	}
	if c.config.Globals.PrintRequest {
		err := c.printMsg(tName, "Get Request:", req)
		if err != nil {
			c.logger.Printf("%v", err)
			if !c.config.Globals.Log {
				fmt.Printf("%v\n", err)
			}
		}
	}
	c.logger.Printf("sending gNMI GetRequest: prefix='%v', path='%v', type='%v', encoding='%v', models='%+v', extension='%+v' to %s",
		xreq.Prefix, xreq.Path, xreq.Type, xreq.Encoding, xreq.UseModels, xreq.Extension, tName)
	response, err := c.collector.Get(ctx, tName, xreq)
	if err != nil {
		c.logger.Printf("failed sending GetRequest to %s: %v", tName, err)
		return
	}
	err = c.printMsg(tName, "Get Response:", response)
	if err != nil {
		c.logger.Printf("target %s: %v", tName, err)
		if !c.config.Globals.Log {
			fmt.Printf("target %s: %v\n", tName, err)
		}
	}
}

// used to init or reset getCmd flags for gnmic-prompt mode
func initGetFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&cli.config.LocalFlags.GetPath, "path", "", []string{}, "get request paths")
	cmd.MarkFlagRequired("path")
	cmd.Flags().StringVarP(&cli.config.LocalFlags.GetPrefix, "prefix", "", "", "get request prefix")
	cmd.Flags().StringSliceVarP(&cli.config.LocalFlags.GetModel, "model", "", []string{}, "get request models")
	cmd.Flags().StringVarP(&cli.config.LocalFlags.GetType, "type", "t", "ALL", "data type requested from the target. one of: ALL, CONFIG, STATE, OPERATIONAL")
	cmd.Flags().StringVarP(&cli.config.LocalFlags.GetTarget, "target", "", "", "get request target")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		cli.config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func (c *CLI) getRunE(cmd *cobra.Command, args []string) error {
	if c.config.Globals.Format == "event" {
		return fmt.Errorf("format event not supported for Get RPC")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupCloseHandler(cancel)
	targetsConfig, err := c.config.GetTargets()
	if err != nil {
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
	req, err := c.config.CreateGetRequest()
	if err != nil {
		return err
	}

	c.wg.Add(len(targetsConfig))
	for tName := range targetsConfig {
		go c.getRequest(ctx, tName, req)
	}
	c.wg.Wait()
	return nil
}
