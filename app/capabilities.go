package app

import (
	"context"
	"fmt"

	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) CapPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	a.createCollectorDialOpts()
	return a.initTunnelServer(tunnel.ServerConfig{
		AddTargetHandler:    a.tunServerAddTargetHandler,
		DeleteTargetHandler: a.tunServerDeleteTargetHandler,
		RegisterHandler:     a.tunServerRegisterHandler,
		Handler:             a.tunServerHandler,
	})
}

func (a *App) CapRunE(cmd *cobra.Command, args []string) error {
	defer a.InitCapabilitiesFlags(cmd)

	if a.Config.Format == "event" {
		return fmt.Errorf("format event not supported for Capabilities RPC")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	//
	targetsConfig, err := a.GetTargets()
	if err != nil {
		return err
	}
	if a.PromptMode {
		// prompt mode
		for _, tc := range targetsConfig {
			a.AddTargetConfig(tc)
		}
	}
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*2)
	a.wg.Add(numTargets)
	for _, tc := range a.Config.Targets {
		go a.ReqCapabilities(ctx, tc)
	}
	a.wg.Wait()
	return a.checkErrors()
}

func (a *App) ReqCapabilities(ctx context.Context, tc *types.TargetConfig) {
	defer a.wg.Done()
	ext := make([]*gnmi_ext.Extension, 0) //
	if a.Config.PrintRequest {
		err := a.PrintMsg(tc.Name, "Capabilities Request:", &gnmi.CapabilityRequest{
			Extension: ext,
		})
		if err != nil {
			a.logError(fmt.Errorf("target %q: %v", tc.Name, err))
		}
	}

	a.Logger.Printf("sending gNMI CapabilityRequest: gnmi_ext.Extension='%v' to %s", ext, tc.Name)
	response, err := a.ClientCapabilities(ctx, tc, ext...)
	if err != nil {
		a.logError(fmt.Errorf("target %q, capabilities request failed: %v", tc.Name, err))
		return
	}

	err = a.PrintMsg(tc.Name, "Capabilities Response:", response)
	if err != nil {
		a.logError(fmt.Errorf("target %q: %v", tc.Name, err))
	}
}

func (a *App) InitCapabilitiesFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().BoolVarP(&a.Config.LocalFlags.CapabilitiesVersion, "version", "", false, "show gnmi version only")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
