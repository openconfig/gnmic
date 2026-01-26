package app

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (a *App) InitTreeFlags(cmd *cobra.Command) {
	cmd.ResetFlags()
	//
	cmd.Flags().BoolVar(&a.Config.TreeFlat, "flat", false, "print flat commands tree")
	cmd.Flags().BoolVar(&a.Config.TreeDetails, "details", false, "print commands flags")
	//
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func (a *App) RunETree(cmd *cobra.Command, args []string) error {
	if a.Config.TreeFlat {
		treeFlat(a.RootCmd, "")
		return nil
	}
	a.tree(a.RootCmd, "")
	return nil
}

func (a *App) tree(c *cobra.Command, indent string) error {
	fmt.Printf("%s", c.Use)
	if !c.HasSubCommands() {
		if c.HasLocalFlags() && a.Config.TreeDetails {
			sections := make([]string, 0)
			c.LocalFlags().VisitAll(func(flag *pflag.Flag) {
				flagSection := ""
				if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
					flagSection = fmt.Sprintf("[-%s | --%s]", flag.Shorthand, flag.Name)
				} else {
					flagSection = fmt.Sprintf("[--%s]", flag.Name)
				}
				sections = append(sections, flagSection)
			})
			fmt.Printf(" %s", strings.Join(sections, " "))
		}
	}
	fmt.Printf("\n")
	subCmds := c.Commands()
	numSubCommands := len(subCmds)
	for i, subC := range subCmds {
		add := " │   "
		if i == numSubCommands-1 {
			fmt.Print(indent + " └─── ")
			add = "     "
		} else {
			fmt.Print(indent + " ├─── ")
		}

		err := a.tree(subC, indent+add)
		if err != nil {
			return err
		}
	}
	return nil
}

func treeFlat(c *cobra.Command, prefix string) {
	prefix += " " + c.Use
	fmt.Println(prefix)
	for _, subC := range c.Commands() {
		treeFlat(subC, prefix)
	}
}
