package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/itchyny/gojq"
	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/formatters"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) GetSetPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	a.Config.LocalFlags.GetSetModel = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GetSetModel)

	a.createCollectorDialOpts()
	return a.initTunnelServer()
}
func (a *App) GetSetRunE(cmd *cobra.Command, args []string) error {
	defer a.InitGetSetFlags(cmd)

	if a.Config.Format == "event" {
		return fmt.Errorf("format event not supported for GetSet RPC")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// setupCloseHandler(cancel)
	targetsConfig, err := a.Config.GetTargets()
	if err != nil {
		return fmt.Errorf("failed getting targets config: %v", err)
	}

	if !a.PromptMode {
		for _, tc := range targetsConfig {
			a.AddTargetConfig(tc)
		}
	}
	req, err := a.Config.CreateGASGetRequest()
	if err != nil {
		return err
	}
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*3)
	a.wg.Add(numTargets)
	for tName := range a.Config.Targets {
		go a.GetSetRequest(ctx, tName, req)
	}
	a.wg.Wait()
	return a.checkErrors()
}

func (a *App) GetSetRequest(ctx context.Context, tName string, req *gnmi.GetRequest) {
	defer a.wg.Done()
	xreq := req
	if len(a.Config.LocalFlags.GetSetModel) > 0 {
		spModels, unspModels, err := a.filterModels(ctx, tName, a.Config.LocalFlags.GetSetModel)
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
	//
	q, err := gojq.Parse(a.Config.LocalFlags.GetSetCondition)
	if err != nil {
		a.logError(err)
		return
	}
	code, err := gojq.Compile(q)
	if err != nil {
		a.logError(err)
		return
	}
	mo := formatters.MarshalOptions{Format: "json"}
	b, err := mo.Marshal(response, map[string]string{"address": tName})
	if err != nil {
		a.logError(fmt.Errorf("error marshaling message: %v", err))
		return
	}
	var input interface{}
	err = json.Unmarshal(b, &input)
	if err != nil {
		a.logError(fmt.Errorf("error unmarshaling message: %v", err))
		return
	}
	iter := code.Run(input)
	var ok bool
	res, ok := iter.Next()
	if !ok {
		a.logError(fmt.Errorf("unexpected jq result type: %v", res))
		// iterator not done, so the final result won't be a boolean
		return
	}
	if err, ok = res.(error); ok {
		if err != nil {
			a.logError(fmt.Errorf("condition evaluation failed: %v", err))
			return
		}
	}
	switch res := res.(type) {
	case bool:
		a.Logger.Printf("GetSet condition evaluated to %v", res)
		if res {
			setReq, err := a.Config.CreateGASSetRequest(input)
			if err != nil {
				a.logError(err)
				return
			}
			if len(setReq.Delete) == 0 && len(setReq.Replace) == 0 && len(setReq.Update) == 0 {
				a.Logger.Printf("empty set request")
				return
			}
			a.setRequest(ctx, tName, setReq)
		}
		return
	default:
		a.logError(errors.New("unexpected condition return type"))
		return
	}
}

// InitGetSetFlags used to init or reset getsetCmd flags for gnmic-prompt mode
func (a *App) InitGetSetFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetSetGet, "get", "", "", "get request paths")
	cmd.MarkFlagRequired("get")
	cmd.Flags().StringSliceVarP(&a.Config.LocalFlags.GetSetModel, "model", "", []string{}, "get request models")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetSetPrefix, "prefix", "", "", "get request prefix")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetSetType, "type", "t", "ALL", "data type requested from the target. one of: ALL, CONFIG, STATE, OPERATIONAL")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetSetTarget, "target", "", "", "get request target")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetSetCondition, "condition", "", "any([true])", "condition to be met in order to execute the set request")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetSetUpdate, "update", "", "", "set update path template, a Go template or a jq expression")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetSetReplace, "replace", "", "", "set replace path template, a Go template or a jq expression")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetSetDelete, "delete", "", "", "set delete path template, a Go template or a jq expression")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GetSetValue, "value", "", "", "set value template, a Go template or a jq expression")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
