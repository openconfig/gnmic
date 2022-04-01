package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/itchyny/gojq"
	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) GetSetPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	a.Config.LocalFlags.GetSetModel = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GetSetModel)

	a.createCollectorDialOpts()
	return a.initTunnelServer(tunnel.ServerConfig{
		AddTargetHandler:    a.tunServerAddTargetHandler,
		DeleteTargetHandler: a.tunServerDeleteTargetHandler,
		RegisterHandler:     a.tunServerRegisterHandler,
		Handler:             a.tunServerHandler,
	})
}
func (a *App) GetSetRunE(cmd *cobra.Command, args []string) error {
	defer a.InitGetSetFlags(cmd)

	if a.Config.Format == "event" {
		return fmt.Errorf("format event not supported for GetSet RPC")
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
	req, err := a.Config.CreateGASGetRequest()
	if err != nil {
		return err
	}
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*3)
	a.wg.Add(numTargets)
	for _, tc := range a.Config.Targets {
		go a.GetSetRequest(ctx, tc, req)
	}
	a.wg.Wait()
	return a.checkErrors()
}

func (a *App) GetSetRequest(ctx context.Context, tc *types.TargetConfig, req *gnmi.GetRequest) {
	defer a.wg.Done()
	xreq := req
	if len(a.Config.LocalFlags.GetSetModel) > 0 {
		spModels, unspModels, err := a.filterModels(ctx, tc, a.Config.LocalFlags.GetSetModel)
		if err != nil {
			a.logError(fmt.Errorf("failed getting supported models from %q: %v", tc.Name, err))
			return
		}
		if len(unspModels) > 0 {
			a.logError(fmt.Errorf("found unsupported models for target %q: %+v", tc.Name, unspModels))
		}
		for _, m := range spModels {
			xreq.UseModels = append(xreq.UseModels, m)
		}
	}
	if a.Config.PrintRequest {
		err := a.PrintMsg(tc.Name, "Get Request:", req)
		if err != nil {
			a.logError(fmt.Errorf("target %q Get Request printing failed: %v", tc.Name, err))
		}
	}
	a.Logger.Printf("sending gNMI GetRequest: prefix='%v', path='%v', type='%v', encoding='%v', models='%+v', extension='%+v' to %s",
		xreq.Prefix, xreq.Path, xreq.Type, xreq.Encoding, xreq.UseModels, xreq.Extension, tc.Name)
	response, err := a.ClientGet(ctx, tc, xreq)
	if err != nil {
		a.logError(fmt.Errorf("target %q get request failed: %v", tc.Name, err))
		return
	}
	err = a.PrintMsg(tc.Name, "Get Response:", response)
	if err != nil {
		a.logError(fmt.Errorf("target %q: %v", tc.Name, err))
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
	b, err := mo.Marshal(response, map[string]string{"address": tc.Name})
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
			a.setRequest(ctx, tc, setReq)
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
