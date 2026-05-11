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
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
)

const (
	initLockerRetryTimer = 1 * time.Second
)

func (a *App) SubscribePreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)

	err := a.initPluginManager()
	if err != nil {
		return err
	}

	a.createCollectorDialOpts()
	return nil
}

func (a *App) SubscribeRunE(cmd *cobra.Command, args []string) error {
	defer a.InitSubscribeFlags(cmd)

	// prompt mode
	if a.PromptMode {
		return a.SubscribeRunPrompt(cmd, args)
	}
	//
	subCfg, err := a.Config.GetSubscriptions(cmd)
	if err != nil {
		return fmt.Errorf("failed reading subscriptions config: %v", err)
	}

	err = a.readConfigs()
	if err != nil {
		return err
	}
	err = a.Config.GetClustering()
	if err != nil {
		return err
	}
	err = a.Config.GetGNMIServer()
	if err != nil {
		return err
	}
	err = a.Config.GetAPIServer()
	if err != nil {
		return err
	}
	err = a.Config.GetLoader()
	if err != nil {
		return err
	}
	numInputs := len(a.Config.Inputs)
	if len(subCfg) == 0 && numInputs == 0 {
		return errors.New("no subscriptions or inputs configuration found")
	}
	// only once mode subscriptions requested
	if allSubscriptionsModeOnce(subCfg) {
		return a.SubscribeRunONCE(cmd, args)
	}
	// only poll mode subscriptions requested
	if allSubscriptionsModePoll(subCfg) {
		return a.SubscribeRunPoll(cmd, args)
	}
	// stream subscriptions
	err = a.initTunnelServer(tunnel.ServerConfig{
		AddTargetHandler:    a.tunServerAddTargetSubscribeHandler,
		DeleteTargetHandler: a.tunServerDeleteTargetHandler,
		RegisterHandler:     a.tunServerRegisterHandler,
		Handler:             a.tunServerHandler,
	})
	if err != nil {
		return err
	}
	_, err = a.Config.GetTargets()
	if errors.Is(err, config.ErrNoTargetsFound) {
		if !a.Config.LocalFlags.SubscribeWatchConfig &&
			len(a.Config.FileConfig.GetStringMap("loader")) == 0 &&
			!a.Config.UseTunnelServer &&
			numInputs == 0 {
			return fmt.Errorf("failed reading targets config: %v", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed reading targets config: %v", err)
	}

	//
	for {
		err := a.InitLocker()
		if err != nil {
			a.Logger.Printf("failed to init locker: %v", err)
			time.Sleep(initLockerRetryTimer)
			continue
		}
		break
	}

	a.startAPIServer()
	a.startGnmiServer()
	go a.startCluster()
	a.startIO()

	if a.Config.LocalFlags.SubscribeWatchConfig {
		go a.watchConfig()
	}

	for range a.ctx.Done() {
		return a.ctx.Err()
	}
	return nil
}

func (a *App) subscribeStream(ctx context.Context, tc *types.TargetConfig) {
	defer a.wg.Done()
	a.TargetSubscribeStream(ctx, tc)
}

func (a *App) subscribeOnce(ctx context.Context, tc *types.TargetConfig) {
	defer a.wg.Done()
	err := a.TargetSubscribeOnce(ctx, tc)
	if err != nil {
		a.logError(err)
	}
}

func (a *App) subscribePoll(ctx context.Context, tc *types.TargetConfig) {
	defer a.wg.Done()
	a.TargetSubscribePoll(ctx, tc)
}

// InitSubscribeFlags used to init or reset subscribeCmd flags for gnmic-prompt mode
func (a *App) InitSubscribeFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().StringVarP(&a.Config.LocalFlags.SubscribePrefix, "prefix", "", "", "subscribe request prefix")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.SubscribePath, "path", "", []string{}, "subscribe request paths")
	//cmd.MarkFlagRequired("path")
	cmd.Flags().Uint32VarP(&a.Config.LocalFlags.SubscribeQos, "qos", "q", 0, "qos marking")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SubscribeUpdatesOnly, "updates-only", "", false, "only updates to current state should be sent")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SubscribeMode, "mode", "", "stream", "one of: once, stream, poll")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SubscribeStreamMode, "stream-mode", "", "target-defined", "one of: on-change, sample, target-defined")
	cmd.Flags().DurationVarP(&a.Config.LocalFlags.SubscribeSampleInterval, "sample-interval", "i", 0,
		"sample interval as a decimal number and a suffix unit, such as \"10s\" or \"1m30s\"")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SubscribeSuppressRedundant, "suppress-redundant", "", false, "suppress redundant update if the subscribed value didn't not change")
	cmd.Flags().DurationVarP(&a.Config.LocalFlags.SubscribeHeartbeatInterval, "heartbeat-interval", "", 0, "heartbeat interval in case suppress-redundant is enabled")
	cmd.Flags().StringSliceVarP(&a.Config.LocalFlags.SubscribeModel, "model", "", []string{}, "subscribe request used model(s)")
	cmd.Flags().BoolVar(&a.Config.LocalFlags.SubscribeQuiet, "quiet", false, "suppress stdout printing")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SubscribeTarget, "target", "", "", "subscribe request target")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SubscribeSetTarget, "set-target", "", false, "set target name in gNMI Path prefix")
	cmd.Flags().StringSliceVarP(&a.Config.LocalFlags.SubscribeName, "name", "n", []string{}, "reference subscriptions by name, must be defined in gnmic config file")
	cmd.Flags().StringSliceVarP(&a.Config.LocalFlags.SubscribeOutput, "output", "", []string{}, "reference to output groups by name, must be defined in gnmic config file")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SubscribeWatchConfig, "watch-config", "", false, "watch configuration changes, add or delete subscribe targets accordingly")
	cmd.Flags().DurationVarP(&a.Config.LocalFlags.SubscribeBackoff, "backoff", "", 0, "backoff time between subscribe requests")
	cmd.Flags().DurationVarP(&a.Config.LocalFlags.SubscribeLockRetry, "lock-retry", "", 5*time.Second, "time to wait between target lock attempts")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SubscribeHistorySnapshot, "history-snapshot", "", "", "sets the snapshot time in a historical subscription, nanoseconds since Unix epoch or RFC3339 format")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SubscribeHistoryStart, "history-start", "", "", "sets the start time in a historical range subscription, nanoseconds since Unix epoch or RFC3339 format")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.SubscribeHistoryEnd, "history-end", "", "", "sets the end time in a historical range subscription, nanoseconds since Unix epoch or RFC3339 format")
	cmd.Flags().Uint32VarP(&a.Config.LocalFlags.SubscribeDepth, "depth", "", 0, "depth extension value")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.SubscribeDryRun, "dry-run", "", false, "dry-run mode, only print the subscribe request")
	//
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func (a *App) readConfigs() error {
	var err error
	_, err = a.Config.GetOutputs()
	if err != nil {
		return fmt.Errorf("failed reading outputs config: %v", err)
	}
	_, err = a.Config.GetInputs()
	if err != nil {
		return fmt.Errorf("failed reading inputs config: %v", err)
	}
	_, err = a.Config.GetActions()
	if err != nil {
		return fmt.Errorf("failed reading actions config: %v", err)
	}
	_, err = a.Config.GetEventProcessors()
	if err != nil {
		return fmt.Errorf("failed reading event processors config: %v", err)
	}
	_, err = a.LoadProtoFiles()
	if err != nil {
		return fmt.Errorf("failed loading proto files: %v", err)
	}
	return nil
}

const (
	subscriptionModeONCE = "ONCE"
	subscriptionModePOLL = "POLL"
)

func (a *App) StartTargetsManager(ctx context.Context) {
	defer func() {
		for _, o := range a.Outputs {
			o.Close()
		}
	}()

	for t := range a.targetsChan {
		if a.Config.Debug {
			a.Logger.Printf("starting target %+v", t)
		}
		if t == nil {
			continue
		}
		a.operLock.RLock()
		_, ok := a.activeTargets[t.Config.Name]
		a.operLock.RUnlock()
		if ok {
			if a.Config.Debug {
				a.Logger.Printf("target %q listener already active", t.Config.Name)
			}
			continue
		}
		a.operLock.Lock()
		a.activeTargets[t.Config.Name] = struct{}{}
		a.operLock.Unlock()

		a.Logger.Printf("starting target %q listener", t.Config.Name)
		go func(t *target.Target) {
			numOnceSubscriptions := t.NumberOfOnceSubscriptions()
			remainingOnceSubscriptions := numOnceSubscriptions
			numSubscriptions := len(t.Subscriptions)
			rspChan, errChan := t.ReadSubscriptions()
			for {
				select {
				case rsp := <-rspChan:
					subscribeResponseReceivedCounter.WithLabelValues(t.Config.Name, rsp.SubscriptionConfig.Name).Add(1)
					if a.Config.Debug {
						a.Logger.Printf("target %q: gNMI Subscribe Response: %+v", t.Config.Name, rsp)
					}
					err := t.DecodeProtoBytes(rsp.Response)
					if err != nil {
						a.Logger.Printf("target %q: failed to decode proto bytes: %v", t.Config.Name, err)
						continue
					}
					m := outputs.Meta{
						"source":            t.Config.Name,
						"format":            a.Config.Format,
						"subscription-name": rsp.SubscriptionName,
					}
					if rsp.SubscriptionConfig.Target != "" {
						m["subscription-target"] = rsp.SubscriptionConfig.Target
					}
					for k, v := range t.Config.EventTags {
						m[k] = v
					}

					// Allow overridden outputs per subscription
					// If both target and subscription have a specified Output, the subscription's Output will be used
					var outs []string
					if len(rsp.SubscriptionConfig.Outputs) > 0 {
						outs = rsp.SubscriptionConfig.Outputs
					} else {
						outs = t.Config.Outputs
					}

					a.export(ctx, rsp.Response, m, outs...)
					if remainingOnceSubscriptions > 0 {
						if a.subscriptionMode(rsp.SubscriptionName) == subscriptionModeONCE {
							switch rsp.Response.Response.(type) {
							case *gnmi.SubscribeResponse_SyncResponse:
								remainingOnceSubscriptions--
							}
						}
					}
					if remainingOnceSubscriptions == 0 && numSubscriptions == numOnceSubscriptions {
						a.operLock.Lock()
						delete(a.activeTargets, t.Config.Name)
						a.operLock.Unlock()
						return
					}
				case tErr := <-errChan:
					if errors.Is(tErr.Err, io.EOF) {
						a.Logger.Printf("target %q: subscription %s closed stream(EOF)", t.Config.Name, tErr.SubscriptionName)
					} else {
						subscribeResponseFailedCounter.WithLabelValues(t.Config.Name, tErr.SubscriptionName).Inc()
						a.Logger.Printf("target %q: subscription %s rcv error: %v", t.Config.Name, tErr.SubscriptionName, tErr.Err)
					}
					if remainingOnceSubscriptions > 0 {
						if a.subscriptionMode(tErr.SubscriptionName) == subscriptionModeONCE {
							remainingOnceSubscriptions--
						}
					}
					if remainingOnceSubscriptions == 0 && numSubscriptions == numOnceSubscriptions {
						a.operLock.Lock()
						delete(a.activeTargets, t.Config.Name)
						a.operLock.Unlock()
						return
					}
				case <-t.StopChan:
					a.operLock.Lock()
					delete(a.activeTargets, t.Config.Name)
					a.operLock.Unlock()
					a.Logger.Printf("target %q: listener stopped", t.Config.Name)
					return
				case <-ctx.Done():
					a.operLock.Lock()
					delete(a.activeTargets, t.Config.Name)
					a.operLock.Unlock()
					return
				}
			}
		}(t)
	}
	for range ctx.Done() {
		return
	}
}

func (a *App) export(ctx context.Context, rsp *gnmi.SubscribeResponse, m outputs.Meta, outs ...string) {
	if rsp == nil {
		return
	}
	go a.updateCache(ctx, rsp, m)
	wg := new(sync.WaitGroup)
	// target has no explicitly defined outputs
	if len(outs) == 0 {
		wg.Add(len(a.Outputs))
		for _, o := range a.Outputs {
			go func(o outputs.Output) {
				defer wg.Done()
				defer a.operLock.RUnlock()
				a.operLock.RLock()
				o.Write(ctx, rsp, m)
			}(o)
		}
		wg.Wait()
		return
	}
	// write to the outputs defined under the target
	for _, name := range outs {
		a.operLock.RLock()
		if o, ok := a.Outputs[name]; ok {
			wg.Add(1)
			go func(o outputs.Output) {
				defer wg.Done()
				o.Write(ctx, rsp, m)
			}(o)
		}
		a.operLock.RUnlock()
	}
	wg.Wait()
}

func (a *App) updateCache(ctx context.Context, rsp *gnmi.SubscribeResponse, m outputs.Meta) {
	if a.c == nil {
		return
	}
	r := proto.Clone(rsp).(*gnmi.SubscribeResponse)
	switch r := r.Response.(type) {
	case *gnmi.SubscribeResponse_Update:
		if r.Update.GetPrefix() == nil {
			r.Update.Prefix = new(gnmi.Path)
		}
		if r.Update.GetPrefix().GetTarget() == "" {
			r.Update.Prefix.Target = utils.GetHost(m["source"])
		}
		target := r.Update.GetPrefix().GetTarget()
		if target == "" {
			a.Logger.Printf("response missing target")
			return
		}
		if a.Config.Debug {
			a.Logger.Printf("updating target %q cache", target)
		}
		sub := m["subscription-name"]
		a.c.Write(ctx, sub, &gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_Update{Update: r.Update}})
	}
}

func (a *App) subscriptionMode(name string) string {
	if sub, ok := a.Config.Subscriptions[name]; ok {
		return strings.ToUpper(sub.Mode)
	}
	return ""
}

// polledSubscriptionsTargets returns a map of target name to a list of subscription names that have Mode == POLL
func (a *App) polledSubscriptionsTargets() map[string][]string {
	result := make(map[string][]string)
	for tn, target := range a.Targets {
		for _, sub := range target.Subscriptions {
			if strings.ToUpper(sub.Mode) == subscriptionModePOLL {
				if result[tn] == nil {
					result[tn] = make([]string, 0)
				}
				result[tn] = append(result[tn], sub.Name)
			}
		}
	}
	return result
}

func (a *App) handlePolledSubscriptions() error {
	polledTargetsSubscriptions := a.polledSubscriptionsTargets()

	if len(polledTargetsSubscriptions) == 0 {
		return nil
	}
	mo := &formatters.MarshalOptions{
		Multiline:        true,
		Indent:           "  ",
		Format:           a.Config.Format,
		CalculateLatency: a.Config.GlobalFlags.CalculateLatency,
	}
	// handle initial responses if updates-only is not set
	if !a.Config.SubscribeUpdatesOnly {
		for targetName := range polledTargetsSubscriptions {
			a.operLock.RLock()
			t, ok := a.Targets[targetName]
			a.operLock.RUnlock()
			if !ok {
				return fmt.Errorf("unknown target name %q", targetName)
			}
			rspCh, errCh := t.ReadSubscriptions()
		SUBS:
			for {
				select {
				case rsp := <-rspCh:
					b, err := mo.Marshal(rsp.Response, nil)
					if err != nil {
						return fmt.Errorf("target '%s', subscription '%s': poll response formatting error: %v", targetName, rsp.SubscriptionName, err)
					}
					fmt.Println(string(b))
					switch rsp := rsp.Response.Response.(type) {
					case *gnmi.SubscribeResponse_SyncResponse:
						fmt.Printf("received sync response '%t' from '%s'\n", rsp.SyncResponse, targetName)
						break SUBS // current target done sending initial updates
					}
				case tErr := <-errCh:
					if tErr.Err != nil {
						return fmt.Errorf("target '%s', subscription '%s': poll response error: %v", targetName, tErr.SubscriptionName, tErr.Err)
					}
				case <-a.ctx.Done():
					return a.ctx.Err()
				}
			}
		}
	}

	pollTargets := make([]string, 0, len(polledTargetsSubscriptions))
	for t := range polledTargetsSubscriptions {
		pollTargets = append(pollTargets, t)
	}
	sort.Slice(pollTargets, func(i, j int) bool {
		return pollTargets[i] < pollTargets[j]
	})
	s := promptui.Select{
		Label:        "select target to poll",
		Items:        pollTargets,
		HideSelected: true,
	}
	waitChan := make(chan struct{}, 1)
	waitChan <- struct{}{}

OUTER:
	for {
		select {
		case <-waitChan:
			_, name, err := s.Run()
			if err != nil {
				fmt.Printf("failed selecting target to poll: %v\n", err)
				continue
			}
			ss := promptui.Select{
				Label:        "select subscription to poll",
				Items:        polledTargetsSubscriptions[name],
				HideSelected: true,
			}
			_, subName, err := ss.Run()
			if err != nil {
				fmt.Printf("failed selecting subscription to poll: %v\n", err)
				continue
			}
			err = a.clientSubscribePoll(a.Context(), name, subName)
			if err != nil && err != io.EOF {
				fmt.Printf("target '%s', subscription '%s': poll response error:%v\n", name, subName, err)
				continue
			}
			a.operLock.RLock()
			t, ok := a.Targets[name]
			a.operLock.RUnlock()
			if !ok {
				return fmt.Errorf("unknown target name %q", name)
			}
			rspCh, errCh := t.ReadSubscriptions()
			for {
				select {
				case <-a.Context().Done():
					return a.Context().Err()
				case tErr := <-errCh:
					if tErr.Err != nil {
						fmt.Printf("received error from target '%s': %v\n", name, err)
						waitChan <- struct{}{}
						continue OUTER
					}
				case rsp, ok := <-rspCh:
					if !ok {
						waitChan <- struct{}{}
						continue OUTER
					}
					if rsp == nil {
						fmt.Printf("received empty response from target '%s'\n", name)
						continue
					}
					switch rsp := rsp.Response.Response.(type) {
					case *gnmi.SubscribeResponse_SyncResponse:
						fmt.Printf("received sync response '%t' from '%s'\n", rsp.SyncResponse, name)
						waitChan <- struct{}{}
						continue OUTER
					}
					b, err := mo.Marshal(rsp.Response, nil)
					if err != nil {
						fmt.Printf("target '%s', subscription '%s': poll response formatting error:%v\n", name, subName, err)
						fmt.Println(rsp.Response)
						continue
					}
					fmt.Println(string(b))
				}
			}
		case <-a.ctx.Done():
			return a.Context().Err()
		}
	}

}

func (a *App) startIO() {
	go a.StartTargetsManager(a.ctx)
	a.InitOutputs(a.ctx)
	a.InitInputs(a.ctx)

	if !a.inCluster() {
		go a.startLoader(a.ctx)
		var limiter *time.Ticker
		if a.Config.LocalFlags.SubscribeBackoff > 0 {
			limiter = time.NewTicker(a.Config.LocalFlags.SubscribeBackoff)
		}

		if !a.Config.UseTunnelServer {
			for _, tc := range a.Config.Targets {
				a.wg.Add(1)
				go a.subscribeStream(a.ctx, tc)
				if limiter != nil {
					<-limiter.C
				}
			}
		}
		if limiter != nil {
			limiter.Stop()
		}
		a.wg.Wait()
	}
}

func allSubscriptionsModeOnce(subs map[string]*types.SubscriptionConfig) bool {
	if len(subs) == 0 {
		return false
	}
	for _, sub := range subs {
		if strings.ToUpper(sub.Mode) != "ONCE" {
			return false
		}
	}
	return true
}

func allSubscriptionsModePoll(subs map[string]*types.SubscriptionConfig) bool {
	if len(subs) == 0 {
		return false
	}
	for _, sub := range subs {
		if strings.ToUpper(sub.Mode) != "POLL" {
			return false
		}
	}
	return true
}
