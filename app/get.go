package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) GetPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	a.Config.LocalFlags.GetPath = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GetPath)
	a.Config.LocalFlags.GetModel = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GetModel)
	a.Config.LocalFlags.GetProcessor = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GetProcessor)

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
	req, err := a.Config.CreateGetRequest()
	if err != nil {
		return err
	}
	// event format
	if len(a.Config.GetProcessor) > 0 {
		a.Config.Format = "event"
	}
	if a.Config.Format == "event" {
		return a.handleGetRequestEvent(ctx, req, evps)
	}
	// other formats
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*3)
	a.wg.Add(numTargets)
	for _, tc := range a.Config.Targets {
		go a.GetRequest(ctx, tc, req)
	}
	a.wg.Wait()
	err = a.checkErrors()
	if err != nil {
		return err
	}
	return nil
}

func (a *App) GetRequest(ctx context.Context, tc *types.TargetConfig, req *gnmi.GetRequest) {
	defer a.wg.Done()
	response, err := a.getRequest(ctx, tc, req)
	if err != nil {
		a.logError(fmt.Errorf("target %q get request failed: %v", tc.Name, err))
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
		return nil, err
	}
	return response, nil
}

func (a *App) filterModels(ctx context.Context, tc *types.TargetConfig, modelsNames []string) (map[string]*gnmi.ModelData, []string, error) {
	supModels, err := a.GetModels(ctx, tc)
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
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GetValuesOnly, "values-only", "", false, "print GetResponse values only")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.GetProcessor, "processor", "", []string{}, "list of processor names to run")

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

func (a *App) handleGetRequestEvent(ctx context.Context, req *gnmi.GetRequest, evps []formatters.EventProcessor) error {
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets*3)
	a.wg.Add(numTargets)
	rsps := make(chan *getResponseEvents)
	for _, tc := range a.Config.Targets {
		go func(tc *types.TargetConfig) {
			defer a.wg.Done()
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
	responses := make(map[string][]*formatters.EventMsg)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case r, ok := <-rsps:
				if !ok {
					return
				}
				responses[r.name] = r.rsp
			}
		}
	}()
	a.wg.Wait()
	close(rsps)
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
