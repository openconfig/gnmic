// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
)

func (a *App) SetPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	err := a.Config.ValidateSetInput()
	if err != nil {
		return err
	}

	a.createCollectorDialOpts()
	return a.initTunnelServer(tunnel.ServerConfig{
		AddTargetHandler:    a.tunServerAddTargetHandler,
		DeleteTargetHandler: a.tunServerDeleteTargetHandler,
		RegisterHandler:     a.tunServerRegisterHandler,
		Handler:             a.tunServerHandler,
	})
}

func (a *App) SetRunE(cmd *cobra.Command, args []string) error {
	defer a.InitSetFlags(cmd)

	if a.Config.Format == formatEvent {
		return fmt.Errorf("format event not supported for Set RPC")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// setupCloseHandler(cancel)
	targetsConfig, err := a.GetTargets()
	if err != nil {
		return fmt.Errorf("failed getting targets config: %v", err)
	}
	if !a.PromptMode {
		for _, tc := range targetsConfig {
			a.AddTargetConfig(tc)
		}
	}
	err = a.Config.ReadSetRequestTemplate()
	if err != nil {
		return fmt.Errorf("failed reading set request files: %v", err)
	}
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*2)
	a.wg.Add(numTargets)
	for _, tc := range a.Config.Targets {
		go a.SetRequest(ctx, tc)
	}
	a.wg.Wait()
	return a.checkErrors()
}

func (a *App) SetRequest(ctx context.Context, tc *types.TargetConfig) {
	defer a.wg.Done()
	reqs, err := a.Config.CreateSetRequest(tc.Name)
	if err != nil {
		a.logError(fmt.Errorf("target %q: failed to create set request: %v", tc.Name, err))
		return
	}
	for _, req := range reqs {
		a.setRequest(ctx, tc, req)
	}
}

func (a *App) setRequest(ctx context.Context, tc *types.TargetConfig, req *gnmi.SetRequest) {
	a.Logger.Printf("sending gNMI SetRequest: prefix='%v', delete='%v', replace='%v', update='%v', extension='%v' to %s",
		req.Prefix, req.Delete, req.Replace, req.Update, req.Extension, tc.Name)
	if a.Config.PrintRequest || a.Config.SetDryRun {
		err := a.PrintMsg(tc.Name, "Set Request:", req)
		if err != nil {
			a.logError(fmt.Errorf("target %q: %v", tc.Name, err))
		}
	}
	if a.Config.SetDryRun {
		return
	}
	response, err := a.ClientSet(ctx, tc, req)
	if err != nil {
		a.logError(fmt.Errorf("target %q set request failed: %v", tc.Name, err))
		return
	}
	err = a.PrintMsg(tc.Name, "Set Response:", response)
	if err != nil {
		a.logError(fmt.Errorf("target %q: %v", tc.Name, err))
	}
}

// InitSetFlags used to init or reset setCmd flags for gnmic-prompt mode
func (a *App) InitSetFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetPrefix, "prefix", "", "", "set request prefix")

	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetDelete, "delete", "", []string{}, "set request path to be deleted")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetReplace, "replace", "", []string{}, fmt.Sprintf("set request path:::type:::value to be replaced, type must be one of %v", config.ValueTypes))
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUpdate, "update", "", []string{}, fmt.Sprintf("set request path:::type:::value to be updated, type must be one of %v", config.ValueTypes))
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUnionReplace, "union-replace", "", []string{}, fmt.Sprintf("set request path:::type:::value to be union-replaced, type must be one of %v", config.ValueTypes))

	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetReplacePath, "replace-path", "", []string{}, "set request path to be replaced")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetReplaceValue, "replace-value", "", []string{}, "set replace request value")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetReplaceFile, "replace-file", "", []string{}, "set replace request value in a json/yaml file")

	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUpdatePath, "update-path", "", []string{}, "set request path to be updated")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUpdateFile, "update-file", "", []string{}, "set update request value in a json/yaml file")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUpdateValue, "update-value", "", []string{}, "set update request value")

	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUnionReplacePath, "union-replace-path", "", []string{}, "set request path for a union_replace")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUnionReplaceValue, "union-replace-value", "", []string{}, "set request union_replace value")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUnionReplaceFile, "union-replace-file", "", []string{}, "set request union_replace value in a json/yaml file")

	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetDelimiter, "delimiter", "", ":::", "set update/replace delimiter between path, type, value")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetTarget, "target", "", "", "set request target")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetRequestFile, "request-file", "", []string{}, "set request template file(s)")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetRequestVars, "request-vars", "", "", "set request variables file")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SetDryRun, "dry-run", "", false, "prints the set request without initiating a gRPC connection")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetRequestProtoFile, "set-proto-request-file", "", []string{}, "set request from prototext file")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SetNoTrim, "no-trim", "", false, "won't trim the input files")
	//
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetReplaceCli, "replace-cli", "", []string{}, "a cli command to be sent as a set replace request")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetReplaceCliFile, "replace-cli-file", "", "", "path to a file containing a list of commands that will be sent as a set replace request")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUpdateCli, "update-cli", "", []string{}, "a cli command to be sent as a set update request")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetUpdateCliFile, "update-cli-file", "", "", "path to a file containing a list of commands that will be sent as a set update request")

	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetCommitId, "commit-id", "", "", "commit ID value")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SetCommitRequest, "commit-request", "", false, "start a commit confirmed transaction")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SetCommitConfirm, "commit-confirm", "", false, "confirm the commit ID")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SetCommitCancel, "commit-cancel", "", false, "cancel the commit")
	cmd.Flags().DurationVarP(&a.Config.LocalFlags.SetCommitRollbackDuration, "rollback-duration", "", 0, "set the commit rollback duration")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
