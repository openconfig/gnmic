// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/formatters"
)

func (a *App) GetPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	a.Config.LocalFlags.GetPath = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GetPath)
	a.Config.LocalFlags.GetModel = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GetModel)
	a.Config.LocalFlags.GetProcessor = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GetProcessor)

	err := a.initPluginManager()
	if err != nil {
		return err
	}

	a.createCollectorDialOpts()
	return a.initTunnelServer(tunnel.ServerConfig{
		AddTargetHandler:    a.tunServerAddTargetHandler,
		DeleteTargetHandler: a.tunServerDeleteTargetHandler,
		RegisterHandler:     a.tunServerRegisterHandler,
		Handler:             a.tunServerHandler,
	})
}

func (a *App) GetRun(cmd *cobra.Command, args []string) error {
	defer a.InitGetFlags(cmd)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// setupCloseHandler(cancel)
	targetsConfig, err := a.GetTargets()
	if err != nil {
		return fmt.Errorf("failed getting targets config: %v", err)
	}
	_, err = a.Config.GetActions()
	if err != nil {
		return fmt.Errorf("failed reading actions config: %v", err)
	}
	evps, err := a.intializeEventProcessors()
	if err != nil {
		return fmt.Errorf("failed to init event processors: %v", err)
	}
	if a.PromptMode {
		// prompt mode
		for _, tc := range targetsConfig {
			a.AddTargetConfig(tc)
		}
	}
	// event format
	if len(a.Config.GetProcessor) > 0 {
		a.Config.Format = formatEvent
	}
	if a.Config.Format == formatEvent {
		return a.handleGetRequestEvent(ctx, evps)
	}
	// other formats
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*3)
	a.wg.Add(numTargets)
	for _, tc := range a.Config.Targets {
		go a.GetRequest(ctx, tc)
	}
	a.wg.Wait()
	err = a.checkErrors()
	if err != nil {
		return err
	}
	return nil
}

func (a *App) GetRequest(ctx context.Context, tc *types.TargetConfig) {
	defer a.wg.Done()
	req, err := a.Config.CreateGetRequest(tc)
	if err != nil {
		a.logError(fmt.Errorf("target %q building Get request failed: %v", tc.Name, err))
		return
	}
	response, err := a.getRequest(ctx, tc, req)
	if err != nil {
		a.logError(fmt.Errorf("target %q Get request failed: %v", tc.Name, err))
		return
	}
	if response == nil {
		return
	}
	err = a.PrintMsg(tc.Name, "Get Response:", response)
	if err != nil {
		a.logError(fmt.Errorf("target %q: %v", tc.Name, err))
	}
}

func (a *App) getRequest(ctx context.Context, tc *types.TargetConfig, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	xreq := req
	if len(a.Config.LocalFlags.GetModel) > 0 {
		spModels, unspModels, err := a.filterModels(ctx, tc, a.Config.LocalFlags.GetModel)
		if err != nil {
			a.logError(fmt.Errorf("failed getting supported models from %q: %v", tc.Name, err))
			return nil, err
		}
		if len(unspModels) > 0 {
			a.logError(fmt.Errorf("found unsupported models for target %q: %+v", tc.Name, unspModels))
		}
		for _, m := range spModels {
			xreq.UseModels = append(xreq.UseModels, m)
		}
	}
	if a.Config.PrintRequest || a.Config.GetDryRun {
		err := a.PrintMsg(tc.Name, "Get Request:", req)
		if err != nil {
			a.logError(fmt.Errorf("target %q Get Request printing failed: %v", tc.Name, err))
		}
	}
	if a.Config.GetDryRun {
		return nil, nil
	}
	a.Logger.Printf("sending gNMI GetRequest: prefix='%v', path='%v', type='%v', encoding='%v', models='%+v', extension='%+v' to %s",
		xreq.Prefix, xreq.Path, xreq.Type, xreq.Encoding, xreq.UseModels, xreq.Extension, tc.Name)

	response, err := a.ClientGet(ctx, tc, xreq)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (a *App) getModels(ctx context.Context, tc *types.TargetConfig) ([]*gnmi.ModelData, error) {
	capRsp, err := a.ClientCapabilities(ctx, tc)
	if err != nil {
		return nil, err
	}
	return capRsp.GetSupportedModels(), nil
}

func (a *App) filterModels(ctx context.Context, tc *types.TargetConfig, modelsNames []string) (map[string]*gnmi.ModelData, []string, error) {
	supModels, err := a.getModels(ctx, tc)
	if err != nil {
		return nil, nil, err
	}
	unsupportedModels := make([]string, 0)
	supportedModels := make(map[string]*gnmi.ModelData)
	var found bool
	for _, m := range modelsNames {
		found = false

		modelName := m
		var organization *string
		var version *string

		if strings.Contains(modelName, "/") {
			parts := strings.SplitN(modelName, "/", 2)
			organization = &parts[0]
			modelName = parts[1]
		}

		if strings.Contains(modelName, ":") {
			parts := strings.SplitN(modelName, ":", 2)
			modelName = parts[0]
			version = &parts[1]
		}

		for _, tModel := range supModels {
			if modelName == tModel.Name && (organization == nil || *organization == tModel.Organization) && (version == nil || *version == tModel.Version) {
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
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GetValuesOnly, "values-only", "", false, "print GetResponse values only")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.GetProcessor, "processor", "", []string{}, "list of processor names to run")
	cmd.Flags().Uint32VarP(&a.Config.LocalFlags.GetDepth, "depth", "", 0, "depth extension value")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GetDryRun, "dry-run", "", false, "prints the get request without initiating a gRPC connection")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func (a *App) intializeEventProcessors() ([]formatters.EventProcessor, error) {
	_, err := a.Config.GetEventProcessors()
	if err != nil {
		return nil, fmt.Errorf("failed reading event processors config: %v", err)
	}
	var evps = make([]formatters.EventProcessor, 0)
	for _, epName := range a.Config.GetProcessor {
		if epCfg, ok := a.Config.Processors[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType],
					formatters.WithLogger(a.Logger),
					formatters.WithTargets(a.Config.Targets),
					formatters.WithActions(a.Config.Actions),
				)
				if err != nil {
					return nil, fmt.Errorf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
				}
				evps = append(evps, ep)
				continue
			}
			return nil, fmt.Errorf("%q event processor has an unknown type=%q", epName, epType)
		}
		return nil, fmt.Errorf("%q event processor not found", epName)
	}
	return evps, nil
}

func (a *App) handleGetRequestEvent(ctx context.Context, evps []formatters.EventProcessor) error {
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*3)
	a.wg.Add(numTargets)
	rsps := make(chan *getResponseEvents, numTargets)
	for _, tc := range a.Config.Targets {
		go func(tc *types.TargetConfig) {
			defer a.wg.Done()
			req, err := a.Config.CreateGetRequest(tc)
			if err != nil {
				a.errCh <- err
				return
			}
			resp, err := a.getRequest(ctx, tc, req)
			if err != nil {
				a.errCh <- err
				return
			}
			evs, err := formatters.GetResponseToEventMsgs(resp, map[string]string{"source": tc.Name}, evps...)
			if err != nil {
				a.errCh <- err
			}
			rsps <- &getResponseEvents{name: tc.Name, rsp: evs}
		}(tc)
	}
	a.wg.Wait()
	close(rsps)

	responses := make(map[string][]*formatters.EventMsg)
	for r := range rsps {
		responses[r.name] = r.rsp
	}
	err := a.checkErrors()
	if err != nil {
		return err
	}
	//
	sb := strings.Builder{}
	for name, r := range responses {
		sb.Reset()
		printPrefix := ""
		if len(a.Config.TargetsList()) > 1 && !a.Config.NoPrefix {
			printPrefix = fmt.Sprintf("[%s] ", name)
		}
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		sb.Write(b)
		fmt.Fprintf(a.out, "%s\n", indent(printPrefix, sb.String()))
	}

	return nil
}

type getResponseEvents struct {
	// target name
	name string
	rsp  []*formatters.EventMsg
}
