package app

import (
	"context"
	"fmt"

	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetReplacePath, "replace-path", "", []string{}, "set request path to be replaced")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUpdatePath, "update-path", "", []string{}, "set request path to be updated")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUpdateFile, "update-file", "", []string{}, "set update request value in json/yaml file")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetReplaceFile, "replace-file", "", []string{}, "set replace request value in json/yaml file")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetUpdateValue, "update-value", "", []string{}, "set update request value")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetReplaceValue, "replace-value", "", []string{}, "set replace request value")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetDelimiter, "delimiter", "", ":::", "set update/replace delimiter between path, type, value")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetTarget, "target", "", "", "set request target")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SetRequestFile, "request-file", "", []string{}, "set request template file(s)")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SetRequestVars, "request-vars", "", "", "set request variables file")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SetDryRun, "dry-run", "", false, "prints the set request without initiating a gRPC connection")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
