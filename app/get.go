package app

import (
	"context"
	"fmt"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) GetRun(cmd *cobra.Command, args []string) error {
	defer a.InitGetFlags(cmd)

	if a.Config.Format == "event" {
		return fmt.Errorf("format event not supported for Get RPC")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// setupCloseHandler(cancel)
	targetsConfig, err := a.Config.GetTargets()
	if err != nil {
		return fmt.Errorf("failed getting targets config: %v", err)
	}

	if a.PromptMode {
		// prompt mode
		for _, tc := range targetsConfig {
			a.AddTargetConfig(tc)
		}
	}
	req, err := a.Config.CreateGetRequest()
	if err != nil {
		return err
	}
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*3)
	a.wg.Add(numTargets)
	for tName := range a.Config.Targets {
		go a.GetRequest(ctx, tName, req)
	}
	a.wg.Wait()
	return a.checkErrors()
}

func (a *App) GetRequest(ctx context.Context, tName string, req *gnmi.GetRequest) {
	defer a.wg.Done()
	xreq := req
	if len(a.Config.LocalFlags.GetModel) > 0 {
		spModels, unspModels, err := a.filterModels(ctx, tName, a.Config.LocalFlags.GetModel)
		if err != nil {
			a.logError(fmt.Errorf("failed getting supported models from %q: %v", tName, err))
			return
		}
		if len(unspModels) > 0 {
			a.logError(fmt.Errorf("found unsupported models for target %q: %+v", tName, unspModels))
		}
		for _, m := range spModels {
			xreq.UseModels = append(xreq.UseModels, m)
		}
	}
	if a.Config.PrintRequest {
		err := a.PrintMsg(tName, "Get Request:", req)
		if err != nil {
			a.logError(fmt.Errorf("target %q Get Request printing failed: %v", tName, err))
		}
	}
	a.Logger.Printf("sending gNMI GetRequest: prefix='%v', path='%v', type='%v', encoding='%v', models='%+v', extension='%+v' to %s",
		xreq.Prefix, xreq.Path, xreq.Type, xreq.Encoding, xreq.UseModels, xreq.Extension, tName)
	response, err := a.ClientGet(ctx, tName, xreq)
	if err != nil {
		a.logError(fmt.Errorf("target %q get request failed: %v", tName, err))
		return
	}
	err = a.PrintMsg(tName, "Get Response:", response)
	if err != nil {
		a.logError(fmt.Errorf("target %q: %v", tName, err))
	}
}

func (a *App) filterModels(ctx context.Context, tName string, modelsNames []string) (map[string]*gnmi.ModelData, []string, error) {
	supModels, err := a.GetModels(ctx, tName)
	if err != nil {
		return nil, nil, err
	}
	unsupportedModels := make([]string, 0)
	supportedModels := make(map[string]*gnmi.ModelData)
	var found bool
	for _, m := range modelsNames {
		found = false
		for _, tModel := range supModels {
			if m == tModel.Name {
				supportedModels[m] = tModel
				found = true
				break
			}
		}
		if !found {
			unsupportedModels = append(unsupportedModels, m)
		}
	}
	return supportedModels, unsupportedModels, nil
}

// InitGetFlags used to init or reset getCmd flags for gnmic-prompt mode
func (a *App) InitGetFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.GetPath, "path", "", []string{}, "get request paths")
	cmd.MarkFlagRequired("path")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetPrefix, "prefix", "", "", "get request prefix")
	cmd.Flags().StringSliceVarP(&a.Config.LocalFlags.GetModel, "model", "", []string{}, "get request models")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetType, "type", "t", "ALL", "data type requested from the target. one of: ALL, CONFIG, STATE, OPERATIONAL")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetTarget, "target", "", "", "get request target")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
