package proxy

import (
	"github.com/openconfig/gnmic/pkg/app"
	"github.com/spf13/cobra"
)

// proxyCmd represents the proxy command
func New(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "proxy",
		Short:   "run a gNMI server that proxies gNMI requests towards known targets",
		PreRunE: gApp.ProxyPreRunE,
		RunE:    gApp.ProxyRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			gApp.CleanupPlugins()
		},
		SilenceUsage: true,
	}
	gApp.InitProxyFlags(cmd)
	return cmd
}
