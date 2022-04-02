package app

import (
	"fmt"
	"time"

	"github.com/karimra/gnmic/types"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
)

func (a *App) SubscribeRunONCE(cmd *cobra.Command, args []string, subCfg map[string]*types.SubscriptionConfig) error {
	a.c = nil // todo:
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
	//
	a.InitOutputs(a.ctx)

	var limiter *time.Ticker
	if a.Config.LocalFlags.SubscribeBackoff > 0 {
		limiter = time.NewTicker(a.Config.LocalFlags.SubscribeBackoff)
	}
	numTargets := len(a.Config.Targets)
	a.errCh = make(chan error, numTargets)
	a.wg.Add(numTargets)
	for _, tc := range a.Config.Targets {
		go a.subscribeOnce(a.ctx, tc)
		if limiter != nil {
			<-limiter.C
		}
	}
	if limiter != nil {
		limiter.Stop()
	}
	a.wg.Wait()
	return a.checkErrors()
}
