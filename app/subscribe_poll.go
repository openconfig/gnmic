// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

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
