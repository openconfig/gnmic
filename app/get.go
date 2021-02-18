package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/karimra/gnmic/collector"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
)

func (a *App) GetRun(cmd *cobra.Command, args []string) error {
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

	if a.collector == nil {
		cfg := &collector.Config{
			Debug:               a.Config.Debug,
			Format:              a.Config.Format,
			TargetReceiveBuffer: a.Config.TargetBufferSize,
			RetryTimer:          a.Config.Retry,
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
	a.collector.InitTargets()
	numTargets := len(a.collector.Targets)
	errCh := make(chan error, numTargets*2)
	a.wg.Add(numTargets)
	for tName := range a.collector.Targets {
		go a.GetRequest(ctx, tName, req, errCh)
	}
	a.wg.Wait()
	close(errCh)
	errs := make([]error, 0, numTargets*2)
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil
	}
	for _, err := range errs {
		fmt.Fprintln(os.Stderr, err)
	}
	return errors.New("one or more get requests failed")
}

func (a *App) GetRequest(ctx context.Context, tName string, req *gnmi.GetRequest, errCh chan error) {
	defer a.wg.Done()
	xreq := req
	if len(a.Config.LocalFlags.GetModel) > 0 {
		spModels, unspModels, err := a.filterModels(ctx, tName, a.Config.LocalFlags.GetModel)
		if err != nil {
			a.Logger.Printf("failed getting supported models from %q: %v", tName, err)
			if !a.Config.Log {
				fmt.Fprintf(os.Stderr, "failed getting supported models from %q: %v\n", tName, err)
			}
			errCh <- err
			return
		}
		if len(unspModels) > 0 {
			a.Logger.Printf("found unsupported models for target %q: %+v", tName, unspModels)
			if !a.Config.Log {
				fmt.Fprintf(os.Stderr, "found unsupported models for target %q: %v\n", tName, err)
			}
		}
		for _, m := range spModels {
			xreq.UseModels = append(xreq.UseModels, m)
		}
	}
	if a.Config.PrintRequest {
		err := a.Print(tName, "Get Request:", req)
		if err != nil {
			a.Logger.Printf("target %q Get Request printing failed: %v", tName, err)
			errCh <- err
			if !a.Config.Log {
				fmt.Fprintf(os.Stderr, "target %q Get Request printing failed: %v\n", tName, err)
			}
		}
	}
	a.Logger.Printf("sending gNMI GetRequest: prefix='%v', path='%v', type='%v', encoding='%v', models='%+v', extension='%+v' to %s",
		xreq.Prefix, xreq.Path, xreq.Type, xreq.Encoding, xreq.UseModels, xreq.Extension, tName)
	response, err := a.collector.Get(ctx, tName, xreq)
	if err != nil {
		errCh <- err
		a.Logger.Printf("target %q get request failed: %v", tName, err)
		if !a.Config.Log {
			fmt.Fprintf(os.Stderr, "target %q get request failed: %v\n", tName, err)
		}
		return
	}
	err = a.Print(tName, "Get Response:", response)
	if err != nil {
		errCh <- err
		a.Logger.Printf("target %q: %v", tName, err)
		if !a.Config.Log {
			fmt.Fprintf(os.Stderr, "target %q: %v\n", tName, err)
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
