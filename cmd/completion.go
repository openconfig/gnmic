package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// getCmd represents the get command
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "completion [bash|zsh|fish]",
		Short:        "generate completion script",
		SilenceUsage: true,
		Long: `To load completions:,

	Bash:

	$ source <(gnmic completion bash)

	# To load completions for each session, execute once:
	# Linux:
	$ gnmic completion bash > /etc/bash_completion.d/gnmic
	# macOS:
	$ gnmic completion bash > /usr/local/etc/bash_completion.d/gnmic

	Zsh:

	# If shell completion is not already enabled in your environment,
	# you will need to enable it.  You can execute the following once:

	$ echo "autoload -U compinit; compinit" >> ~/.zshrc

	# To load completions for each session, execute once:
	$ gnmic completion zsh > "${fpath[1]}/gnmic"

	# You will need to start a new shell for this setup to take effect.

	fish:

	$ gnmic completion fish | source

	# To load completions for each session, execute once:
	$ gnmic completion fish > ~/.config/fish/completions/gnmic.fish
	`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				cmd.Root().GenFishCompletion(os.Stdout, true)
			}
		},
	}
	return cmd
}
