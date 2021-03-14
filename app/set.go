package app

import (
	"context"
	"fmt"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/config"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) SetRun(cmd *cobra.Command, args []string) error {
	defer a.InitSetFlags(cmd)

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
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*2)
	a.wg.Add(numTargets)
	for tName := range a.Config.Targets {
		go a.SetRequest(ctx, tName, req)
	}
	a.wg.Wait()
	return a.checkErrors()
}

func (a *App) SetRequest(ctx context.Context, tName string, req *gnmi.SetRequest) {
	defer a.wg.Done()
	a.Logger.Printf("sending gNMI SetRequest: prefix='%v', delete='%v', replace='%v', update='%v', extension='%v' to %s",
		req.Prefix, req.Delete, req.Replace, req.Update, req.Extension, tName)
	if a.Config.PrintRequest {
		err := a.PrintMsg(tName, "Set Request:", req)
		if err != nil {
			a.logError(fmt.Errorf("target %q: %v", tName, err))
		}
	}
	response, err := a.collector.Set(ctx, tName, req)
	if err != nil {
		a.logError(fmt.Errorf("target %q set request failed: %v", tName, err))
		return
	}
	err = a.PrintMsg(tName, "Set Response:", response)
	if err != nil {
		a.logError(fmt.Errorf("target %q: %v", tName, err))
	}
}

// InitSetFlags used to init or reset setCmd flags for gnmic-prompt mode
func (a *App) InitSetFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().StringP("prefix", "", "", "set request prefix")

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

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
