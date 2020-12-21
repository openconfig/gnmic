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
	"os"
	"sync"

	"github.com/karimra/gnmic/collector"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"

	"github.com/spf13/cobra"
)

// capabilitiesCmd represents the capabilities command
func newCapabilitiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "capabilities",
		Aliases: []string{"cap"},
		Short:   "query targets gnmi capabilities",
		PreRun: func(cmd *cobra.Command, args []string) {
			cfg.SetLocalFlagsFromFile(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.Globals.Format == "event" {
				return fmt.Errorf("format event not supported for Capabilities RPC")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			setupCloseHandler(cancel)
			targetsConfig, err := cfg.GetTargets()
			if err != nil {
				return fmt.Errorf("failed getting targets config: %v", err)
			}
			if cfg.Globals.Debug {
				logger.Printf("targets: %s", targetsConfig)
			}
			if coll == nil {
				cfg := &collector.Config{
					Debug:               cfg.Globals.Debug,
					Format:              cfg.Globals.Format,
					TargetReceiveBuffer: cfg.Globals.TargetBufferSize,
					RetryTimer:          cfg.Globals.Retry,
				}

				coll = collector.NewCollector(cfg, targetsConfig,
					collector.WithDialOptions(createCollectorDialOpts()),
					collector.WithLogger(logger),
				)
			} else {
				// prompt mode
				for _, tc := range targetsConfig {
					coll.AddTarget(tc)
				}
			}

			wg := new(sync.WaitGroup)
			wg.Add(len(coll.Targets))
			lock := new(sync.Mutex)
			for tName := range coll.Targets {
				go reqCapabilities(ctx, coll, tName, wg, lock)
			}
			wg.Wait()
			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initCapabilitiesFlags(cmd)
		},
		SilenceUsage: true,
	}
	initCapabilitiesFlags(cmd)
	return cmd
}

func reqCapabilities(ctx context.Context, coll *collector.Collector, tName string, wg *sync.WaitGroup, lock *sync.Mutex) {
	defer wg.Done()
	ext := make([]*gnmi_ext.Extension, 0) //
	if cfg.Globals.PrintRequest {
		lock.Lock()
		fmt.Fprint(os.Stderr, "Capabilities Request:\n")
		err := printMsg(tName, &gnmi.CapabilityRequest{
			Extension: ext,
		})
		if err != nil {
			logger.Printf("error marshaling capabilities request: %v", err)
			if !cfg.Globals.Log {
				fmt.Printf("error marshaling capabilities request: %v", err)
			}
		}
		lock.Unlock()
	}

	logger.Printf("sending gNMI CapabilityRequest: gnmi_ext.Extension='%v' to %s", ext, tName)
	response, err := coll.Capabilities(ctx, tName, ext...)
	if err != nil {
		logger.Printf("error sending capabilities request: %v", err)
		return
	}
	lock.Lock()
	defer lock.Unlock()
	fmt.Fprint(os.Stderr, "Capabilities Response:\n")
	err = printMsg(tName, response)
	if err != nil {
		logger.Printf("error marshaling capabilities response from %s: %v", tName, err)
		if !cfg.Globals.Log {
			fmt.Printf("error marshaling capabilities response from %s: %v\n", tName, err)
		}
	}
}

func initCapabilitiesFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&cfg.LocalFlags.CapabilitiesVersion, "version", "", false, "show gnmi version only")
	cfg.FileConfig.BindPFlag("capabilities-version", cmd.LocalFlags().Lookup("version"))
}
