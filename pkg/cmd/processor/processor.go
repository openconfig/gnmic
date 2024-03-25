package processor

import (
	"github.com/openconfig/gnmic/pkg/app"
	"github.com/spf13/cobra"
)

// processorCmd represents the processor command
func New(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "processor",
		Short:   "apply a list of processors",
		PreRunE: gApp.ProcessorPreRunE,
		RunE:    gApp.ProcessorRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			gApp.CleanupPlugins()
		},
		SilenceUsage: true,
	}
	gApp.InitProcessorFlags(cmd)
	return cmd
}
