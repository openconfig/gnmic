package tree

import (
	"github.com/openconfig/gnmic/pkg/app"
	"github.com/spf13/cobra"
)

// New create the subscribe command tree.
func New(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tree",
		Short: "print the commands tree",
		RunE:  gApp.RunETree,
		PostRun: func(cmd *cobra.Command, args []string) {
			gApp.CleanupPlugins()
		},
		SilenceUsage: true,
	}
	gApp.InitTreeFlags(cmd)
	return cmd
}
