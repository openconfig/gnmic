package app

import (
	"context"
	"fmt"

	"github.com/karimra/gnmic/collector"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
)

func (a *App) GetRun(cmd *cobra.Command, args []string) error {
	if a.Config.Globals.Format == "event" {
		return fmt.Errorf("format event not supported for Get RPC")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// setupCloseHandler(cancel)
	targetsConfig, err := a.Config.GetTargets()
	if err != nil {
		return fmt.Errorf("failed getting targets config: %v", err)
	}

	if a.collector == nil {
		cfg := &collector.Config{
			Debug:               a.Config.Globals.Debug,
			Format:              a.Config.Globals.Format,
			TargetReceiveBuffer: a.Config.Globals.TargetBufferSize,
			RetryTimer:          a.Config.Globals.Retry,
		}

		a.collector = collector.NewCollector(cfg, targetsConfig,
			collector.WithDialOptions(a.createCollectorDialOpts()),
			collector.WithLogger(a.Logger),
		)
	} else {
		// prompt mode
		for _, tc := range targetsConfig {
			a.collector.AddTarget(tc)
		}
	}
	req, err := a.Config.CreateGetRequest()
	if err != nil {
		return err
	}

	a.wg.Add(len(targetsConfig))
	for tName := range targetsConfig {
		go a.GetRequest(ctx, tName, req)
	}
	a.wg.Wait()
	return nil
}

func (a *App) GetRequest(ctx context.Context, tName string, req *gnmi.GetRequest) {
	defer a.wg.Done()
	xreq := req
	if len(a.Config.LocalFlags.GetModel) > 0 {
		spModels, unspModels, err := a.filterModels(ctx, tName, a.Config.LocalFlags.GetModel)
		if err != nil {
			a.Logger.Printf("failed getting supported models from '%s': %v", tName, err)
			return
		}
		if len(unspModels) > 0 {
			a.Logger.Printf("found unsupported models for target '%s': %+v", tName, unspModels)
		}
		for _, m := range spModels {
			xreq.UseModels = append(xreq.UseModels, m)
		}
	}
	if a.Config.Globals.PrintRequest {
		err := a.Print(tName, "Get Request:", req)
		if err != nil {
			a.Logger.Printf("%v", err)
			if !a.Config.Globals.Log {
				fmt.Printf("%v\n", err)
			}
		}
	}
	a.Logger.Printf("sending gNMI GetRequest: prefix='%v', path='%v', type='%v', encoding='%v', models='%+v', extension='%+v' to %s",
		xreq.Prefix, xreq.Path, xreq.Type, xreq.Encoding, xreq.UseModels, xreq.Extension, tName)
	response, err := a.collector.Get(ctx, tName, xreq)
	if err != nil {
		a.Logger.Printf("failed sending GetRequest to %s: %v", tName, err)
		return
	}
	err = a.Print(tName, "Get Response:", response)
	if err != nil {
		a.Logger.Printf("target %s: %v", tName, err)
		if !a.Config.Globals.Log {
			fmt.Printf("target %s: %v\n", tName, err)
		}
	}
}

func (a *App) filterModels(ctx context.Context, tName string, modelsNames []string) (map[string]*gnmi.ModelData, []string, error) {
	supModels, err := a.collector.GetModels(ctx, tName)
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
