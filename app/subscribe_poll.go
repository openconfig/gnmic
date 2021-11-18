package app

import (
	"fmt"

	"github.com/karimra/gnmic/types"
	"github.com/spf13/cobra"
)

func (a *App) SubscribeRunPoll(cmd *cobra.Command, args []string, subCfg map[string]*types.SubscriptionConfig) error {
	_, err := a.Config.GetTargets()
	if err != nil {
		return fmt.Errorf("failed reading targets config: %v", err)
	}

	err = a.readConfigs()
	if err != nil {
		return err
	}

	go a.StartCollector(a.ctx)

	a.wg.Add(len(a.Config.Targets))
	for name := range a.Config.Targets {
		go a.subscribePoll(a.ctx, name)
	}
	a.wg.Wait()
	a.handlePolledSubscriptions()
	return nil
}
