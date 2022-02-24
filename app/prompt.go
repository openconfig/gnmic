package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/nsf/termbox-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) PromptRunE(cmd *cobra.Command, args []string) error {
	err := a.generateYangSchema(a.Config.GlobalFlags.Dir, a.Config.GlobalFlags.File, a.Config.GlobalFlags.Exclude)
	if err != nil {
		a.Logger.Printf("failed to load paths from yang: %v", err)
		if !a.Config.Log {
			fmt.Fprintf(os.Stderr, "ERR: failed to load paths from yang: %v\n", err)
		}
	}
	a.PromptMode = true
	// load history
	a.PromptHistory = make([]string, 0, 256)
	home, err := homedir.Dir()
	if err != nil {
		if a.Config.Debug {
			a.Logger.Printf("failed to get home directory: %v", err)
		}
		return nil
	}
	content, err := os.ReadFile(filepath.Join(home, ".gnmic.history"))
	if err != nil {
		if a.Config.Debug {
			a.Logger.Printf("failed to read history file: %v", err)
		}
		return nil
	}
	history := strings.Split(string(content), "\n")
	for i := range history {
		if history[i] != "" {
			a.PromptHistory = append(a.PromptHistory, history[i])
		}
	}
	return nil
}

// PreRun resolve the glob patterns and checks if --max-suggestions is bigger that the terminal height and lowers it if needed.
func (a *App) PromptPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	err := a.yangFilesPreProcessing()
	if err != nil {
		return err
	}
	err = termbox.Init()
	if err != nil {
		return fmt.Errorf("could not initialize a terminal box: %v", err)
	}
	_, h := termbox.Size()
	termbox.Close()
	// set max suggestions to terminal height-1 if the supplied value is greater
	if uint(a.Config.LocalFlags.PromptMaxSuggestions) > uint(h) {
		if h > 1 {
			a.Config.LocalFlags.PromptMaxSuggestions = uint16(h - 2)
		} else {
			a.Config.LocalFlags.PromptMaxSuggestions = 0
		}
	}
	return nil
}

func (a *App) InitPromptFlags(cmd *cobra.Command) {
	cmd.Flags().Uint16Var(&a.Config.LocalFlags.PromptMaxSuggestions, "max-suggestions", 10, "terminal suggestion max list size")
	cmd.Flags().StringVar(&a.Config.LocalFlags.PromptPrefixColor, "prefix-color", "dark_blue", "terminal prefix color")
	cmd.Flags().StringVar(&a.Config.LocalFlags.PromptSuggestionsBGColor, "suggestions-bg-color", "dark_blue", "suggestion box background color")
	cmd.Flags().StringVar(&a.Config.LocalFlags.PromptDescriptionBGColor, "description-bg-color", "dark_gray", "description box background color")
	cmd.Flags().BoolVar(&a.Config.LocalFlags.PromptSuggestAllFlags, "suggest-all-flags", false, "suggest local as well as inherited flags of subcommands")
	cmd.Flags().BoolVar(&a.Config.LocalFlags.PromptDescriptionWithPrefix, "description-with-prefix", false, "show YANG module prefix in XPATH suggestion description")
	cmd.Flags().BoolVar(&a.Config.LocalFlags.PromptDescriptionWithTypes, "description-with-types", false, "show YANG types in XPATH suggestion description")
	cmd.Flags().BoolVar(&a.Config.LocalFlags.PromptSuggestWithOrigin, "suggest-with-origin", false, "suggest XPATHs with origin prepended ")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
