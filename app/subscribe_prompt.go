package app

import (
	"fmt"
	"time"

	"github.com/karimra/gnmic/types"
	"github.com/spf13/cobra"
)

func (a *App) SubscribeRunPrompt(cmd *cobra.Command, args []string) error {
	// stop running subscriptions
	for _, t := range a.Targets {
		t.StopSubscriptions()
	}
	// reset subscriptions config map
	a.Config.Subscriptions = make(map[string]*types.SubscriptionConfig)

	// read targets
	_, err := a.Config.GetTargets()
	if err != nil {
		return fmt.Errorf("failed reading targets config: %v", err)
	}
	subCfg, err := a.Config.GetSubscriptions(cmd)
	if err != nil {
		return fmt.Errorf("failed reading subscriptions config: %v", err)
	}
	// only once mode subscriptions requested
	if allSubscriptionsModeOnce(subCfg) {
		return a.SubscribeRunONCE(cmd, args, subCfg)
	}
	// only poll mode subscriptions requested
	if allSubscriptionsModePoll(subCfg) {
		return a.SubscribeRunPoll(cmd, args, subCfg)
	}
	// stream+once mode subscriptions
	err = a.readConfigs()
	if err != nil {
		return err
	}
	go a.StartCollector(a.ctx)

	a.InitOutputs(a.ctx)

	var limiter *time.Ticker
	if a.Config.LocalFlags.SubscribeBackoff > 0 {
		limiter = time.NewTicker(a.Config.LocalFlags.SubscribeBackoff)
	}

	a.wg.Add(len(a.Config.Targets))
	for _, tc := range a.Config.Targets {
		go a.subscribeStream(a.ctx, tc)
		if limiter != nil {
			<-limiter.C
		}
	}
	if limiter != nil {
		limiter.Stop()
	}
	a.wg.Wait()

	return nil
}
