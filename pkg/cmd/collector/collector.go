// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/openconfig/gnmic/pkg/app"
	"github.com/openconfig/gnmic/pkg/collector"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/zestor-dev/zestor/store"
)

// New create the collector command tree.
func New(gApp *app.App) *cobra.Command {
	c := collector.New(gApp.Context(), gApp.Store)
	cmd := &cobra.Command{
		Use:     "collect",
		Aliases: []string{"c", "coll", "collector"},
		Short:   "collect gNMI telemetry from targets",
		PreRunE: c.CollectorPreRunE,
		RunE:    c.CollectorRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			gApp.CleanupPlugins()
		},
		SilenceUsage: true,
	}
	c.InitCollectorFlags(cmd)
	cmd.AddCommand(newCollectorTargetsCmd(gApp))
	cmd.AddCommand(newCollectorSubscriptionsCmd(gApp))
	cmd.AddCommand(newCollectorOutputsCmd(gApp))
	cmd.AddCommand(newCollectorProcessorsCmd(gApp))
	cmd.AddCommand(newCollectorInputsCmd(gApp))
	return cmd
}

func getAPIServerURL(store store.Store[any]) (string, error) {
	apiServerConfig, ok, err := store.Get("api-server", "api-server")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("api-server config not found")
	}
	apiCfg, ok := apiServerConfig.(*config.APIServer)
	if !ok {
		return "", fmt.Errorf("api-server config is required for collector command")
	}
	if apiCfg == nil {
		return "", fmt.Errorf("api-server config is required for collector command")
	}
	if apiCfg.TLS != nil {
		return "https://" + apiCfg.Address, nil
	}
	return "http://" + apiCfg.Address, nil
}

func getAPIServerClient(store store.Store[any]) (*http.Client, error) {
	apiServerConfig, ok, err := store.Get("api-server", "api-server")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("api-server config not found")
	}
	apiCfg, ok := apiServerConfig.(*config.APIServer)
	if !ok {
		return nil, fmt.Errorf("address not found")
	}
	if apiCfg.TLS != nil {
		return &http.Client{
			Timeout: apiCfg.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}, nil
	}
	return &http.Client{
		Timeout: apiCfg.Timeout,
	}, nil
}
