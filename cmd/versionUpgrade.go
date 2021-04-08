package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// upgradeCmd represents the version command
func newVersionUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "upgrade",
		Aliases: []string{"up"},
		Short:   "upgrade gnmic to latest available version",
		PreRun: func(cmd *cobra.Command, args []string) {
			gApp.Config.SetLocalFlagsFromFile(cmd)
		},
		RunE: gApp.VersionUpgradeRun,
	}
	initVersionUpgradeFlags(cmd)
	return cmd
}

func initVersionUpgradeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("use-pkg", false, "upgrade using package")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		gApp.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
