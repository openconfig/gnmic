package app

import (
	"context"
	"fmt"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) CapPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	a.createCollectorDialOpts()
	return a.initTunnelServer()
}

func (a *App) CapRunE(cmd *cobra.Command, args []string) error {
	defer a.InitCapabilitiesFlags(cmd)

	if a.Config.Format == "event" {
		return fmt.Errorf("format event not supported for Capabilities RPC")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	targetsConfig, err := a.Config.GetTargets()
	if err != nil {
		a.Logger.Printf("failed getting targets config: %v", err)
		return fmt.Errorf("failed getting targets config: %v", err)
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
	for tName := range a.Config.Targets {
		go a.ReqCapabilities(ctx, tName)
	}
	a.wg.Wait()
	return a.checkErrors()
}

func (a *App) ReqCapabilities(ctx context.Context, tName string) {
	defer a.wg.Done()
	ext := make([]*gnmi_ext.Extension, 0) //
	if a.Config.PrintRequest {
		err := a.PrintMsg(tName, "Capabilities Request:", &gnmi.CapabilityRequest{
			Extension: ext,
		})
		if err != nil {
			a.logError(fmt.Errorf("target %q: %v", tName, err))
		}
	}

	a.Logger.Printf("sending gNMI CapabilityRequest: gnmi_ext.Extension='%v' to %s", ext, tName)
	response, err := a.ClientCapabilities(ctx, tName, ext...)
	if err != nil {
		a.logError(fmt.Errorf("target %q, capabilities request failed: %v", tName, err))
		return
	}

	err = a.PrintMsg(tName, "Capabilities Response:", response)
	if err != nil {
		a.logError(fmt.Errorf("target %q: %v", tName, err))
	}
}

func (a *App) InitCapabilitiesFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().BoolVarP(&a.Config.LocalFlags.CapabilitiesVersion, "version", "", false, "show gnmi version only")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
