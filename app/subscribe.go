package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/formatters"
	"github.com/manifoldco/promptui"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
)

const (
	defaultRetryTimer  = 10 * time.Second
	defaultBackoff     = 100 * time.Millisecond
	defaultClusterName = "default-cluster"

	initLockerRetryTimer = 1 * time.Second
)

func (a *App) SubscribeRun(cmd *cobra.Command, args []string) error {
	inputsConfig, err := a.Config.GetInputs()
	if err != nil {
		return fmt.Errorf("failed getting inputs config: %v", err)
	}
	targetsConfig, err := a.Config.GetTargets()
	if (errors.Is(err, config.ErrNoTargetsFound) && !a.Config.LocalFlags.SubscribeWatchConfig) ||
		(!errors.Is(err, config.ErrNoTargetsFound) && err != nil) {
		return fmt.Errorf("failed getting targets config: %v", err)
	}

	subscriptionsConfig, err := a.Config.GetSubscriptions(cmd)
	if err != nil {
		return fmt.Errorf("failed getting subscriptions config: %v", err)
	}
	outs, err := a.Config.GetOutputs()
	if err != nil {
		return err
	}
	epconfig, err := a.Config.GetEventProcessors()
	if err != nil {
		return err
	}
	lockerConfig, err := a.Config.GetLocker()
	if err != nil {
		return err
	}
	//
	for {
		err := a.InitLocker(a.ctx, lockerConfig)
		if err != nil {
			a.Logger.Printf("failed to init locker: %v", err)
			time.Sleep(initLockerRetryTimer)
			continue
		}
		break
	}
	//
	if a.collector == nil {
		cfg := &collector.Config{
			Name:                a.Config.InstanceName,
			PrometheusAddress:   a.Config.PrometheusAddress,
			Debug:               a.Config.Debug,
			Format:              a.Config.Format,
			TargetReceiveBuffer: a.Config.TargetBufferSize,
			RetryTimer:          a.Config.Retry,
			ClusterName:         a.Config.LocalFlags.SubscribeClusterName,
			LockRetryTimer:      a.Config.LocalFlags.SubscribeLockRetry,
		}
		a.Logger.Printf("starting collector with config %+v", cfg)
		a.collector = collector.NewCollector(cfg, targetsConfig,
			collector.WithDialOptions(a.createCollectorDialOpts()),
			collector.WithSubscriptions(subscriptionsConfig),
			collector.WithOutputs(outs),
			collector.WithLogger(a.Logger),
			collector.WithEventProcessors(epconfig),
			collector.WithInputs(inputsConfig),
			collector.WithLocker(lockerConfig),
		)
		go a.collector.Start(a.ctx)
		go a.startAPI()
	} else {
		// prompt mode
		for name, outCfg := range outs {
			err = a.collector.AddOutput(name, outCfg)
			if err != nil {
				a.Logger.Printf("%v", err)
			}
		}
		for _, sc := range subscriptionsConfig {
			err = a.collector.AddSubscriptionConfig(sc)
			if err != nil {
				a.Logger.Printf("%v", err)
			}
		}
		for _, tc := range targetsConfig {
			a.collector.AddTarget(tc)
			if err != nil {
				a.Logger.Printf("%v", err)
			}
		}
	}

	for {
		err := a.collector.InitLocker(a.ctx)
		if err != nil {
			a.Logger.Printf("failed to init locker: %v", err)
			time.Sleep(initLockerRetryTimer)
			continue
		}
		break
	}

	a.collector.InitOutputs(a.ctx)
	a.collector.InitInputs(a.ctx)
	a.collector.InitTargets()

	if lockerConfig != nil && a.Config.LocalFlags.SubscribeBackoff <= 0 {
		a.Config.LocalFlags.SubscribeBackoff = defaultBackoff
	}

	var limiter *time.Ticker
	if a.Config.LocalFlags.SubscribeBackoff > 0 {
		limiter = time.NewTicker(a.Config.LocalFlags.SubscribeBackoff)
	}

	a.wg.Add(len(a.collector.Targets))
	for name := range a.collector.Targets {
		go a.Subscribe(a.ctx, name)
		if limiter != nil {
			<-limiter.C
		}
	}
	if limiter != nil {
		limiter.Stop()
	}
	a.wg.Wait()

	polledTargetsSubscriptions := a.collector.PolledSubscriptionsTargets()
	if len(polledTargetsSubscriptions) > 0 {
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
		mo := &formatters.MarshalOptions{
			Multiline: true,
			Indent:    "  ",
			Format:    a.Config.Format,
		}
		go func() {
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
					response, err := a.collector.TargetPoll(name, subName)
					if err != nil && err != io.EOF {
						fmt.Printf("target '%s', subscription '%s': poll response error:%v\n", name, subName, err)
						continue
					}
					if response == nil {
						fmt.Printf("received empty response from target '%s'\n", name)
						continue
					}
					switch rsp := response.Response.(type) {
					case *gnmi.SubscribeResponse_SyncResponse:
						fmt.Printf("received sync response '%t' from '%s'\n", rsp.SyncResponse, name)
						waitChan <- struct{}{}
						continue
					}
					b, err := mo.Marshal(response, nil)
					if err != nil {
						fmt.Printf("target '%s', subscription '%s': poll response formatting error:%v\n", name, subName, err)
						fmt.Println(string(b))
						waitChan <- struct{}{}
						continue
					}
					fmt.Println(string(b))
					waitChan <- struct{}{}
				case <-a.ctx.Done():
					return
				}
			}
		}()
	}
	if a.Config.LocalFlags.SubscribeWatchConfig {
		go a.watchConfig()
	}

	if a.PromptMode {
		return nil
	}
	for range a.ctx.Done() {
		return a.ctx.Err()
	}
	return nil
}

func (a *App) Subscribe(ctx context.Context, name string) {
	defer a.wg.Done()
	a.collector.StartTarget(ctx, name)
}
