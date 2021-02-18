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

func (a *App) SetRun(cmd *cobra.Command, args []string) error {
	if a.Config.Format == "event" {
		return fmt.Errorf("format event not supported for Set RPC")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// setupCloseHandler(cancel)
	targetsConfig, err := a.Config.GetTargets()
	if err != nil {
		return fmt.Errorf("failed getting targets config: %v", err)
	}
	if len(targetsConfig) > 1 {
		fmt.Println("[warning] running set command on multiple targets")
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
	req, err := a.Config.CreateSetRequest()
	if err != nil {
		return err
	}
	a.collector.InitTargets()
	numTargets := len(a.collector.Targets)
	errCh := make(chan error, numTargets*2)
	a.wg.Add(numTargets)
	for tName := range a.collector.Targets {
		go a.SetRequest(ctx, tName, req, errCh)
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
		fmt.Println(err)
	}
	return errors.New("one or more set requests failed")
}

func (a *App) SetRequest(ctx context.Context, tName string, req *gnmi.SetRequest, errCh chan error) {
	defer a.wg.Done()
	a.Logger.Printf("sending gNMI SetRequest: prefix='%v', delete='%v', replace='%v', update='%v', extension='%v' to %s",
		req.Prefix, req.Delete, req.Replace, req.Update, req.Extension, tName)
	if a.Config.PrintRequest {
		err := a.Print(tName, "Set Request:", req)
		if err != nil {
			errCh <- err
			a.Logger.Printf("target %q: %v", tName, err)
			if !a.Config.Log {
				fmt.Fprintf(os.Stderr, "target %q: %v\n", tName, err)
			}
		}
	}
	response, err := a.collector.Set(ctx, tName, req)
	if err != nil {
		errCh <- err
		a.Logger.Printf("target %q set request failed: %v", tName, err)
		if !a.Config.Log {
			fmt.Fprintf(os.Stderr, "target %q set request failed: %v\n", tName, err)
		}
		return
	}
	err = a.Print(tName, "Set Response:", response)
	if err != nil {
		errCh <- err
		a.Logger.Printf("target %q: %v", tName, err)
		if !a.Config.Log {
			fmt.Fprintf(os.Stderr, "target %q: %v\n", tName, err)
		}
	}
}
