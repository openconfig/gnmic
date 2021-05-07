package app

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) GeneratePathPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	if a.Config.LocalFlags.GeneratePathPathType != "xpath" && a.Config.LocalFlags.GeneratePathPathType != "gnmi" {
		return fmt.Errorf("path-type must be one of 'xpath' or 'gnmi'")
	}
	return nil
}

func (a *App) GeneratePathRunE(cmd *cobra.Command, args []string) error {
	return a.PathCmdRun(
		a.Config.GlobalFlags.Dir,
		a.Config.GlobalFlags.File,
		a.Config.GlobalFlags.Exclude,
		a.Config.GeneratePathSearch,
		a.Config.GeneratePathWithDescr,
		a.Config.GeneratePathWithPrefix,
		a.Config.GeneratePathWithTypes,
		a.Config.GeneratePathPathType,
		a.Config.GeneratePathState,
		a.Config.GeneratePathConfig,
	)
}

func (a *App) InitGeneratePathFlags(cmd *cobra.Command) {
	cmd.ResetFlags()
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GeneratePathPathType, "path-type", "", "xpath", "path type xpath or gnmi")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GeneratePathWithDescr, "descr", "", false, "print leaf description")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GeneratePathWithPrefix, "with-prefix", "", false, "include module/submodule prefix in path elements")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GeneratePathWithTypes, "types", "", false, "print leaf type")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GeneratePathSearch, "search", "", false, "search through path list")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GeneratePathState, "state", "", false, "generate paths only for YANG leafs representing state data")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GeneratePathConfig, "config", "", false, "generate paths only for YANG leafs representing config data")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
