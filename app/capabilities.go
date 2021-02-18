package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/karimra/gnmic/collector"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/spf13/cobra"
)

func (a *App) CapRun(cmd *cobra.Command, args []string) error {
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
	a.collector.InitTargets()
	numTargets := len(a.collector.Targets)
	errCh := make(chan error, numTargets*2)
	a.wg.Add(numTargets)
	for tName := range a.collector.Targets {
		go a.ReqCapabilities(ctx, tName, errCh)
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
	return errors.New("one or more capabilities requests failed")
}

func (a *App) ReqCapabilities(ctx context.Context, tName string, errCh chan error) {
	defer a.wg.Done()
	ext := make([]*gnmi_ext.Extension, 0) //
	if a.Config.PrintRequest {
		err := a.Print(tName, "Capabilities Request:", &gnmi.CapabilityRequest{
			Extension: ext,
		})
		if err != nil {
			errCh <- err
			a.Logger.Printf("target %q: %v", tName, err)
			if !a.Config.Log {
				fmt.Fprintf(os.Stderr, "target %q: %v\n", tName, err)
			}
		}
	}

	a.Logger.Printf("sending gNMI CapabilityRequest: gnmi_ext.Extension='%v' to %s", ext, tName)
	response, err := a.collector.Capabilities(ctx, tName, ext...)
	if err != nil {
		errCh <- err
		a.Logger.Printf("target %q, capabilities request failed: %v", tName, err)
		if !a.Config.Log {
			fmt.Fprintf(os.Stderr, "target %q, capabilities request failed: %v\n", tName, err)
		}
		return
	}

	err = a.Print(tName, "Capabilities Response:", response)
	if err != nil {
		errCh <- err
		a.Logger.Printf("target %q: %v", tName, err)
		if !a.Config.Log {
			fmt.Fprintf(os.Stderr, "target %q: %v\n", tName, err)
		}
	}
}
