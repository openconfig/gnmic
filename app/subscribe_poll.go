package app

import (
	"fmt"

	"github.com/karimra/gnmic/types"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
)

func (a *App) SubscribeRunPoll(cmd *cobra.Command, args []string, subCfg map[string]*types.SubscriptionConfig) error {
	a.initTunnelServer(tunnel.ServerConfig{
		AddTargetHandler:    a.tunServerAddTargetHandler,
		DeleteTargetHandler: a.tunServerDeleteTargetHandler,
		RegisterHandler:     a.tunServerRegisterHandler,
		Handler:             a.tunServerHandler,
	})
	_, err := a.GetTargets()
	if err != nil {
		return fmt.Errorf("failed reading targets config: %v", err)
	}

	err = a.readConfigs()
	if err != nil {
		return err
	}

	go a.StartCollector(a.ctx)

	a.wg.Add(len(a.Config.Targets))
	for _, tc := range a.Config.Targets {
		go a.subscribePoll(a.ctx, tc)
	}
	a.wg.Wait()
	a.handlePolledSubscriptions()
	return nil
}
