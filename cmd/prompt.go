package cmd

import (
	"fmt"
	"os"
	"strings"

	goprompt "github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/nsf/termbox-go"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var promptMode bool
var schemaTree *yang.Entry
var promptModeCmd = &cobra.Command{
	Use:     "prompt-mode",
	Aliases: []string{"prompt"},
	Short:   "enter the interactive gnmic prompt-mode",
	RunE: func(cmd *cobra.Command, args []string) error {
		if promptMode {
			return fmt.Errorf("already entered to the prompt-mode")
		}
		promptMode = true
		return nil
	},
	SilenceUsage: true,
}

var promptQuitCmd = &cobra.Command{
	Use:   "quit",
	Short: "quit the gnmic-prompt",
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(promptModeCmd)
}

func findMatchedXPATH(entry *yang.Entry, word string, cursor int) []goprompt.Suggest {
	suggestions := make([]goprompt.Suggest, 0, 4)
	cword := word[cursor:]
	for name, child := range entry.Dir {
		pathelem := "/" + name
		if strings.HasPrefix(pathelem, cword) {
			node := ""
			if os.PathSeparator != '/' {
				node = fmt.Sprintf("%s%s", word[:cursor], pathelem)
				suggestions = append(suggestions, goprompt.Suggest{Text: node, Description: child.Description})
			} else {
				if len(cword) >= 1 && cword[0] == '/' {
					node = name
				} else {
					node = pathelem
				}
				suggestions = append(suggestions, goprompt.Suggest{Text: node, Description: child.Description})
			}
			if child.Key != "" { // list
				keylist := strings.Split(child.Key, " ")
				for _, key := range keylist {
					node = fmt.Sprintf("%s[%s=*]", node, key)
				}
				suggestions = append(suggestions, goprompt.Suggest{Text: node, Description: child.Description})
			}
		} else if strings.HasPrefix(cword, pathelem) {
			var prevC rune
			var bracketCount int
			var endIndex int = -1
			var stop bool
			for i, c := range cword {
				switch c {
				case '[':
					bracketCount++
				case ']':
					if prevC != '\\' {
						bracketCount--
						endIndex = i
					}
				case '/':
					if i != 0 && bracketCount == 0 {
						endIndex = i
						stop = true
					}
				}
				if stop {
					break
				}
				prevC = c
			}
			if bracketCount == 0 {
				if endIndex >= 0 {
					suggestions = append(suggestions, findMatchedXPATH(child, word, cursor+endIndex)...)
				} else {
					suggestions = append(suggestions, findMatchedXPATH(child, word, cursor+len(pathelem))...)
				}
			}
		}
	}
	return suggestions
}

var filePathCompleter = completer.FilePathCompleter{
	IgnoreCase: true,
	Filter: func(fi os.FileInfo) bool {
		fmt.Println(fi.Name())
		return fi.IsDir() || !strings.HasPrefix(fi.Name(), ".")
	},
}

var yangPathCompleter = completer.FilePathCompleter{
	IgnoreCase: true,
	Filter: func(fi os.FileInfo) bool {
		fmt.Println(fi.Name())
		return fi.IsDir() || strings.HasSuffix(fi.Name(), ".yang")
	},
}

var dirPathCompleter = completer.FilePathCompleter{
	IgnoreCase: true,
	Filter: func(fi os.FileInfo) bool {
		return fi.IsDir()
	},
}

func findDynamicSuggestions(annotation string, doc goprompt.Document) []goprompt.Suggest {
	switch annotation {
	case "XPATH":
		word := doc.GetWordBeforeCursor()
		suggestions := make([]goprompt.Suggest, 0, 16)
		for _, entry := range schemaTree.Dir {
			suggestions = append(suggestions, findMatchedXPATH(entry, word, 0)...)
		}
		return suggestions
	case "FILE":
		return filePathCompleter.Complete(doc)
	case "YANG":
		return yangPathCompleter.Complete(doc)
	case "MODEL":
		suggestions := make([]goprompt.Suggest, 0, len(schemaTree.Dir))
		for name := range schemaTree.Dir {
			suggestions = append(suggestions, goprompt.Suggest{Text: name})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "DIR":
		return dirPathCompleter.Complete(doc)
	}
	return []goprompt.Suggest{}
}

func showCommandArguments(b *goprompt.Buffer) {
	doc := b.Document()
	showLocalFlags := false
	command := rootCmd
	args := strings.Fields(doc.CurrentLine())
	if found, _, err := command.Find(args); err == nil {
		if command != found {
			showLocalFlags = true
		}
		command = found
	}
	maxNameLen := 0
	suggestions := make([]goprompt.Suggest, 0, 32)
	if command.HasAvailableSubCommands() {
		for _, c := range command.Commands() {
			if c.Hidden {
				continue
			}
			length := len(c.Name())
			if maxNameLen < length {
				maxNameLen = length
			}
			suggestions = append(suggestions, goprompt.Suggest{Text: c.Name(), Description: c.Short})
		}
	}
	if showLocalFlags {
		addFlags := func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			length := len(flag.Name)
			if maxNameLen < length+2 {
				maxNameLen = length + 2
			}
			suggestions = append(suggestions, goprompt.Suggest{Text: "--" + flag.Name, Description: flag.Usage})
		}
		command.LocalFlags().VisitAll(addFlags)
	}
	suggestions = goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	if len(suggestions) == 0 {
		return
	}
	if err := termbox.Init(); err != nil {
		logger.Fatalf("%v", err)
	}
	w, _ := termbox.Size()
	termbox.Close()
	fmt.Printf("\n")
	maxDescLen := w - maxNameLen - 6
	format := fmt.Sprintf("  %%-%ds : %%-%ds\n", maxNameLen, maxDescLen)
	for i := range suggestions {
		length := len(suggestions[i].Description)
		if length > maxDescLen {
			fmt.Printf(format, suggestions[i].Text, suggestions[i].Description[:maxDescLen])
		} else {
			fmt.Printf(format, suggestions[i].Text, suggestions[i].Description)
		}
	}
	fmt.Printf("\n")
}

// ExecutePrompt load and run gnmic-prompt mode.
func ExecutePrompt() {
	var err error
	schemaTree, err = loadSchemaZip()
	if err != nil {
		schemaTree = buildRootEntry()
	}
	// goprompt.OptionHistory()
	rootCmd.AddCommand(promptQuitCmd)
	rootCmd.RemoveCommand(promptModeCmd)
	shell := &cmdPrompt{
		RootCmd: rootCmd,
		GoPromptOptions: []goprompt.Option{
			goprompt.OptionTitle("gnmic-prompt"),
			goprompt.OptionPrefix("gnmic> "),
			goprompt.OptionMaxSuggestion(5),
			goprompt.OptionPrefixTextColor(goprompt.Yellow),
			// goprompt.OptionPreviewSuggestionTextColor(goprompt.Yellow),
			goprompt.OptionPreviewSuggestionBGColor(goprompt.Black),
			goprompt.OptionSuggestionTextColor(goprompt.White),
			goprompt.OptionSuggestionBGColor(goprompt.Black),
			goprompt.OptionSelectedSuggestionTextColor(goprompt.Black),
			goprompt.OptionSelectedSuggestionBGColor(goprompt.White),
			// goprompt.OptionDescriptionTextColor(goprompt.White),
			goprompt.OptionDescriptionBGColor(goprompt.Yellow),
			goprompt.OptionSelectedDescriptionTextColor(goprompt.Black),
			goprompt.OptionSelectedDescriptionBGColor(goprompt.White),
			goprompt.OptionScrollbarBGColor(goprompt.White),
			goprompt.OptionAddASCIICodeBind(goprompt.ASCIICodeBind{
				ASCIICode: []byte{0x3f}, Fn: showCommandArguments}),
			goprompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator),
		},
	}
	shell.Run()
}

// Reference: https://github.com/stromland/cobra-prompt
// cmdPrompt requires RootCmd to run
type cmdPrompt struct {
	// RootCmd is the start point, all its sub commands and flags will be available as suggestions
	RootCmd *cobra.Command

	// GoPromptOptions is for customize go-prompt
	// see https://github.com/c-bata/go-prompt/blob/master/option.go
	GoPromptOptions []goprompt.Option
}

// Run will automatically generate suggestions for all cobra commands
// and flags defined by RootCmd and execute the selected commands.
func (co cmdPrompt) Run() {
	p := goprompt.New(
		func(in string) {
			promptArgs := strings.Fields(in)
			os.Args = append([]string{os.Args[0]}, promptArgs...)
			if len(promptArgs) > 0 {
				co.RootCmd.Execute()
			}
		},
		func(d goprompt.Document) []goprompt.Suggest {
			return findSuggestions(co, d)
		},
		co.GoPromptOptions...,
	)
	p.Run()
}

func findSuggestions(co cmdPrompt, doc goprompt.Document) []goprompt.Suggest {
	showLocalFlags := false
	command := co.RootCmd
	args := strings.Fields(doc.CurrentLine())
	if found, _, err := command.Find(args); err == nil {
		if command != found {
			showLocalFlags = true
		}
		command = found
	}

	suggestions := make([]goprompt.Suggest, 0, 32)
	if command.HasAvailableSubCommands() {
		for _, c := range command.Commands() {
			if !c.Hidden {
				suggestions = append(suggestions, goprompt.Suggest{Text: c.Name(), Description: c.Short})
			}
		}
	}

	// check flag annotation for the dynamic suggestion
	annotation := ""
	argnum := len(args)
	wordBefore := doc.GetWordBeforeCursor()
	if wordBefore == "" {
		if argnum >= 1 {
			annotation = command.Annotations[args[argnum-1]]
		}
	} else {
		if argnum >= 2 {
			annotation = command.Annotations[args[argnum-2]]
		}
	}
	if annotation != "" {
		return append(suggestions, findDynamicSuggestions(annotation, doc)...)
	}

	if showLocalFlags {
		// load local flags of the command
		addFlags := func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			suggestions = append(suggestions, goprompt.Suggest{Text: "--" + flag.Name, Description: flag.Usage})
		}
		command.LocalFlags().VisitAll(addFlags)
		// command.InheritedFlags().VisitAll(addFlags)
	} else {

		// persistent flags are shown if run.
		addFlags := func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			// if strings.HasPrefix(doc.GetWordBeforeCursor(), "--") {
			// 	suggestions = append(suggestions, goprompt.Suggest{Text: "--" + flag.Name, Description: flag.Usage})
			// } else if strings.HasPrefix(doc.GetWordBeforeCursor(), "-") && flag.Shorthand != "" {
			// 	suggestions = append(suggestions, goprompt.Suggest{Text: "-" + flag.Shorthand, Description: flag.Usage})
			// }
			if strings.HasPrefix(doc.GetWordBeforeCursor(), "-") {
				suggestions = append(suggestions, goprompt.Suggest{Text: "--" + flag.Name, Description: flag.Usage})
			}
		}
		command.LocalFlags().VisitAll(addFlags)
	}
	return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
}
