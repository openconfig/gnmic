package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	goprompt "github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/karimra/gnmic/collector"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/nsf/termbox-go"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var promptMode bool
var promptHistory []string
var schemaTree = &yang.Entry{
	Dir: make(map[string]*yang.Entry),
}
var colorMapping = map[string]goprompt.Color{
	"black":      goprompt.Black,
	"dark_red":   goprompt.DarkRed,
	"dark_green": goprompt.DarkGreen,
	"brown":      goprompt.Brown,
	"dark_blue":  goprompt.DarkBlue,
	"purple":     goprompt.Purple,
	"cyan":       goprompt.Cyan,
	"light_gray": goprompt.LightGray,
	"dark_gray":  goprompt.DarkGray,
	"red":        goprompt.Red,
	"green":      goprompt.Green,
	"yellow":     goprompt.Yellow,
	"blue":       goprompt.Blue,
	"fuchsia":    goprompt.Fuchsia,
	"turquoise":  goprompt.Turquoise,
	"white":      goprompt.White,
}

func getColor(flagName string) goprompt.Color {
	if cgoprompt, ok := colorMapping[viper.GetString(flagName)]; ok {
		return cgoprompt
	}
	defColor := "yellow"
	promptModeCmd.Flags().VisitAll(
		func(f *pflag.Flag) {
			if f.Name == strings.Replace(flagName, "prompt-", "", 1) {
				defColor = f.DefValue
				return
			}
		},
	)
	return colorMapping[defColor]
}

var promptModeCmd = &cobra.Command{
	Use:   "prompt",
	Short: "enter the interactive gnmic prompt mode",
	// PreRun resolve the glob patterns and checks if --max-suggesions is bigger that the terminal height and lowers it if needed.
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		promptDirs, err = resolveGlobs(promptDirs)
		if err != nil {
			return err
		}
		promptFiles, err = resolveGlobs(promptFiles)
		if err != nil {
			return err
		}
		for _, dirpath := range promptDirs {
			expanded, err := yang.PathsWithModules(dirpath)
			if err != nil {
				return err
			}
			if viper.GetBool("debug") {
				for _, fdir := range expanded {
					logger.Printf("adding %s to yang Paths", fdir)
				}
			}
			yang.AddPath(expanded...)
		}
		yfiles, err := findYangFiles(promptFiles)
		if err != nil {
			return err
		}
		promptFiles = make([]string, 0, len(yfiles))
		promptFiles = append(promptFiles, yfiles...)
		if viper.GetBool("debug") {
			for _, file := range promptFiles {
				logger.Printf("loading %s yang file", file)
			}
		}
		err = termbox.Init()
		if err != nil {
			return fmt.Errorf("could not initialize a terminal box: %v", err)
		}
		_, h := termbox.Size()
		termbox.Close()
		// set max suggestions to terminal height-1 if the supplied value is greater
		if viper.GetUint("prompt-max-suggestions") > uint(h) {
			if h > 1 {
				viper.Set("prompt-max-suggestions", h-2)
			} else {
				viper.Set("prompt-max-suggestions", 0)
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := generateYangSchema(promptDirs, promptFiles, promptExcluded)
		if err != nil {
			logger.Printf("failed to load paths from yang: %v", err)
			if !viper.GetBool("log") {
				fmt.Fprintf(os.Stderr, "ERR: failed to load paths from yang: %v\n", err)
			}
		}
		promptMode = true
		// load history
		promptHistory = make([]string, 0, 256)
		home, err := homedir.Dir()
		if err != nil {
			if viper.GetBool("debug") {
				log.Printf("failed to get home directory: %v", err)
			}
			return nil
		}
		content, err := ioutil.ReadFile(home + "/.gnmic.history")
		if err != nil {
			if viper.GetBool("debug") {
				log.Printf("failed to read history file: %v", err)
			}
			return nil
		}
		history := strings.Split(string(content), "\n")
		for i := range history {
			if history[i] != "" {
				promptHistory = append(promptHistory, history[i])
			}
		}
		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		cmd.ResetFlags()
		//initPromptFlags(cmd)
	},
	SilenceUsage: true,
}

var promptQuitCmd = &cobra.Command{
	Use:   "quit",
	Short: "quit the gnmic-prompt",
	Run: func(cmd *cobra.Command, args []string) {
		// save history
		home, err := homedir.Dir()
		if err != nil {
			os.Exit(0)
		}
		f, err := os.Create(home + "/.gnmic.history")
		if err != nil {
			os.Exit(0)
		}
		l := len(promptHistory)
		if l > 128 {
			promptHistory = promptHistory[l-128:]
		}
		for i := range promptHistory {
			f.WriteString(promptHistory[i] + "\n")
		}
		f.Close()
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(promptModeCmd)
	initPromptFlags(promptModeCmd)
}

var promptFiles []string
var promptExcluded []string
var promptDirs []string

// used to init or reset pathCmd flags for gnmic-prompt mode
func initPromptFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&promptFiles, "file", "", []string{}, "path to a yang file or a directory of them to get path auto-completions from")
	cmd.Flags().StringArrayVarP(&promptExcluded, "exclude", "", []string{}, "yang module names to be excluded from path auto-completion generation")
	cmd.Flags().StringArrayVarP(&promptDirs, "dir", "", []string{}, "path to a directory with yang modules used as includes and/or imports")
	cmd.Flags().Uint16("max-suggestions", 10, "terminal suggestion max list size")
	cmd.Flags().String("prefix-color", "dark_blue", "terminal prefix color")
	cmd.Flags().String("suggestions-bg-color", "dark_blue", "suggestion box background color")
	cmd.Flags().String("description-bg-color", "dark_gray", "description box background color")
	cmd.Flags().Bool("suggest-all-flags", false, "suggest local as well as inherited flags of subcommands")
	cmd.Flags().Bool("description-with-prefix", false, "show YANG module prefix in XPATH suggestion description")
	cmd.Flags().Bool("description-with-types", false, "show YANG types in XPATH suggestion description")
	cmd.Flags().Bool("suggest-with-origin", false, "suggest XPATHs with origin prepended ")
	viper.BindPFlag("prompt-file", cmd.LocalFlags().Lookup("file"))
	viper.BindPFlag("prompt-exclude", cmd.LocalFlags().Lookup("exclude"))
	viper.BindPFlag("prompt-dir", cmd.LocalFlags().Lookup("dir"))
	viper.BindPFlag("prompt-max-suggestions", cmd.LocalFlags().Lookup("max-suggestions"))
	viper.BindPFlag("prompt-prefix-color", cmd.LocalFlags().Lookup("prefix-color"))
	viper.BindPFlag("prompt-suggestions-bg-color", cmd.LocalFlags().Lookup("suggestions-bg-color"))
	viper.BindPFlag("prompt-description-bg-color", cmd.LocalFlags().Lookup("description-bg-color"))
	viper.BindPFlag("prompt-suggest-all-flags", cmd.LocalFlags().Lookup("suggest-all-flags"))
	viper.BindPFlag("prompt-description-with-prefix", cmd.LocalFlags().Lookup("description-with-prefix"))
	viper.BindPFlag("prompt-description-with-types", cmd.LocalFlags().Lookup("description-with-types"))
	viper.BindPFlag("prompt-suggest-with-origin", cmd.LocalFlags().Lookup("suggest-with-origin"))
}

func findMatchedXPATH(entry *yang.Entry, input string, prefixPresent bool) []goprompt.Suggest {
	if strings.HasPrefix(input, ":") {
		return nil
	}
	suggestions := make([]goprompt.Suggest, 0, 4)
	inputLen := len(input)
	for i, c := range input {
		if c == ':' && i+1 < inputLen {
			input = input[i+1:]
			inputLen -= (i + 1)
			break
		}
	}

	prependOrigin := viper.GetBool("prompt-suggest-with-origin") && !prefixPresent
	for name, child := range entry.Dir {
		pathelem := "/" + name
		if strings.HasPrefix(pathelem, input) {
			node := ""
			if inputLen == 0 && prependOrigin {
				node = fmt.Sprintf("%s:/%s", entry.Name, name)
			} else if inputLen > 0 && input[0] == '/' {
				node = name
			} else {
				node = pathelem
			}
			suggestions = append(suggestions, goprompt.Suggest{Text: node, Description: buildXPATHDescription(child)})
			if child.Key != "" { // list
				keylist := strings.Split(child.Key, " ")
				for _, key := range keylist {
					node = fmt.Sprintf("%s[%s=*]", node, key)
				}
				suggestions = append(suggestions, goprompt.Suggest{Text: node, Description: buildXPATHDescription(child)})
			}
		} else if strings.HasPrefix(input, pathelem) {
			var prevC rune
			var bracketCount int
			var endIndex int = -1
			var stop bool
			for i, c := range input {
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
					suggestions = append(suggestions, findMatchedXPATH(child, input[endIndex:], prefixPresent)...)
				} else {
					suggestions = append(suggestions, findMatchedXPATH(child, input[len(pathelem):], prefixPresent)...)
				}
			}
		}
	}
	return suggestions
}

func getDescriptionPrefix(entry *yang.Entry) string {
	switch {
	case entry.Dir == nil && entry.ListAttr != nil: // leaf-list
		return "[⋯]"
	case entry.Dir == nil: // leaf
		return "   "
	case entry.ListAttr != nil: // list
		return "[+]"
	default: // container
		return "[+]"
	}
}
func getEntryType(entry *yang.Entry) string {
	if entry.Type != nil {
		return entry.Type.Kind.String()
	}
	return ""
}
func buildXPATHDescription(entry *yang.Entry) string {
	sb := strings.Builder{}
	sb.WriteString(getDescriptionPrefix(entry))
	sb.WriteString(" ")
	sb.WriteString(getPermissions(entry))
	sb.WriteString(" ")
	if viper.GetBool("prompt-description-with-types") {
		n, _ := sb.WriteString(getEntryType(entry))
		if n > 0 {
			sb.WriteString(", ")
		}
	}
	if viper.GetBool("prompt-description-with-prefix") {
		if entry.Prefix != nil {
			sb.WriteString(entry.Prefix.Name)
			sb.WriteString(": ")
		}
	}
	sb.WriteString(entry.Description)
	return sb.String()
}

func getPermissions(entry *yang.Entry) string {
	if entry == nil {
		return "(rw)"
	}
	switch entry.Config {
	case yang.TSTrue:
		return "(rw)"
	case yang.TSFalse:
		return "(ro)"
	case yang.TSUnset:
		return getPermissions(entry.Parent)
	}
	return "(rw)"
}

func findMatchedSchema(entry *yang.Entry, input string) []*yang.Entry {
	schemaNodes := []*yang.Entry{}
	inputLen := len(input)
	for name, child := range entry.Dir {
		pathelem := "/" + name
		if strings.HasPrefix(pathelem, input) {
			node := ""
			if inputLen > 0 && input[0] == '/' {
				node = name
			} else {
				node = pathelem
			}
			schemaNodes = append(schemaNodes, child)
			if child.Key != "" { // list
				keylist := strings.Split(child.Key, " ")
				for _, key := range keylist {
					node = fmt.Sprintf("%s[%s=*]", node, key)
				}
				schemaNodes = append(schemaNodes, child)
			}
		} else if strings.HasPrefix(input, pathelem) {
			var prevC rune
			var bracketCount int
			var endIndex int = -1
			var stop bool
			for i, c := range input {
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
					schemaNodes = append(schemaNodes, findMatchedSchema(child, input[endIndex:])...)
				} else {
					schemaNodes = append(schemaNodes, findMatchedSchema(child, input[len(pathelem):])...)
				}
			}
		}
	}
	return schemaNodes
}

var filePathCompleter = completer.FilePathCompleter{
	IgnoreCase: true,
	Filter: func(fi os.FileInfo) bool {
		return fi.IsDir() || !strings.HasPrefix(fi.Name(), ".")
	},
}

var yangPathCompleter = completer.FilePathCompleter{
	IgnoreCase: true,
	Filter: func(fi os.FileInfo) bool {
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
		line := doc.CurrentLine()
		word := doc.GetWordBeforeCursor()
		suggestions := make([]goprompt.Suggest, 0, 16)
		entries := []*yang.Entry{}
		if index := strings.Index(line, "--prefix"); index >= 0 {
			line = strings.TrimLeft(line[index+8:], " ") // 8 is len("--prefix")
			end := strings.Index(line, " ")
			if end >= 0 {
				line = line[:end]
				lineLen := len(line)
				// remove "origin:" from prefix if present
				for i, c := range line {
					if c == ':' && i+1 < lineLen {
						line = line[i+1:]
						break
					}
				}
				// find yang entries matching the prefix
				for _, entry := range schemaTree.Dir {
					entries = append(entries, findMatchedSchema(entry, line)...)
				}
				// generate suggestions from matching entries
				for _, entry := range entries {
					suggestions = append(suggestions, findMatchedXPATH(entry, word, true)...)
				}
			}
		} else {
			// generate suggestions from yang schema
			for _, entry := range schemaTree.Dir {
				suggestions = append(suggestions, findMatchedXPATH(entry, word, false)...)
			}
		}
		sort.Slice(suggestions, func(i, j int) bool {
			if suggestions[i].Text == suggestions[j].Text {
				return suggestions[i].Description < suggestions[j].Description
			}
			return suggestions[i].Text < suggestions[j].Text
		})
		return suggestions
	case "PREFIX":
		word := doc.GetWordBeforeCursor()
		suggestions := make([]goprompt.Suggest, 0, 16)
		for _, entry := range schemaTree.Dir {
			suggestions = append(suggestions, findMatchedXPATH(entry, word, false)...)
		}
		sort.Slice(suggestions, func(i, j int) bool {
			if suggestions[i].Text == suggestions[j].Text {
				return suggestions[i].Description < suggestions[j].Description
			}
			return suggestions[i].Text < suggestions[j].Text
		})
		return suggestions
	case "FILE":
		return filePathCompleter.Complete(doc)
	case "YANG":
		return yangPathCompleter.Complete(doc)
	case "MODEL":
		suggestions := make([]goprompt.Suggest, 0, len(schemaTree.Dir))
		for name, dir := range schemaTree.Dir {
			if dir != nil {
				suggestions = append(suggestions, goprompt.Suggest{Text: name, Description: dir.Description})
				continue
			}
			suggestions = append(suggestions, goprompt.Suggest{Text: name})
		}
		sort.Slice(suggestions, func(i, j int) bool {
			if suggestions[i].Text == suggestions[j].Text {
				return suggestions[i].Description < suggestions[j].Description
			}
			return suggestions[i].Text < suggestions[j].Text
		})
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "DIR":
		return dirPathCompleter.Complete(doc)
	case "ENCODING":
		suggestions := make([]goprompt.Suggest, 0, len(encodings))
		for _, sugg := range encodings {
			suggestions = append(suggestions, goprompt.Suggest{Text: sugg[0], Description: sugg[1]})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "FORMAT":
		suggestions := make([]goprompt.Suggest, 0, len(formats))
		for _, sugg := range formats {
			suggestions = append(suggestions, goprompt.Suggest{Text: sugg[0], Description: sugg[1]})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "STORE":
		suggestions := make([]goprompt.Suggest, 0, len(dataType))
		for _, sugg := range dataType {
			suggestions = append(suggestions, goprompt.Suggest{Text: sugg[0], Description: sugg[1]})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "SUBSC_MODE":
		suggestions := make([]goprompt.Suggest, 0, len(subscriptionModes))
		for _, sugg := range subscriptionModes {
			suggestions = append(suggestions, goprompt.Suggest{Text: sugg[0], Description: sugg[1]})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "STREAM_MODE":
		suggestions := make([]goprompt.Suggest, 0, len(streamSubscriptionModes))
		for _, sugg := range streamSubscriptionModes {
			suggestions = append(suggestions, goprompt.Suggest{Text: sugg[0], Description: sugg[1]})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "SUBSCRIPTION":
		subs := readSubscriptionsFromCfg()
		suggestions := make([]goprompt.Suggest, 0, len(subs))
		for _, sub := range subs {
			suggestions = append(suggestions, goprompt.Suggest{Text: sub.Name, Description: subscriptionDescription(sub)})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "TARGET":
		targetsConfig := readTargetsFromCfg()
		suggestions := make([]goprompt.Suggest, 0, len(targetsConfig))
		for _, target := range targetsConfig {
			sb := strings.Builder{}
			if target.Name != target.Address {
				sb.WriteString("address=")
				sb.WriteString(target.Address)
				sb.WriteString(", ")
			}
			sb.WriteString("secure=")
			if *target.Insecure {
				sb.WriteString("false")
			} else {
				sb.WriteString(fmt.Sprintf("%v", !(strings.Contains(doc.CurrentLine(), "--insecure"))))
			}
			suggestions = append(suggestions, goprompt.Suggest{Text: target.Name, Description: sb.String()})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "OUTPUT":
		outputGroups := getOutputsFromCfg()
		fmt.Println()
		suggestions := make([]goprompt.Suggest, 0, len(outputGroups))
		for _, sugg := range outputGroups {
			suggestions = append(suggestions, goprompt.Suggest{Text: sugg.name, Description: strings.Join(sugg.types, ", ")})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	}
	return []goprompt.Suggest{}
}
func subscriptionDescription(sub *collector.SubscriptionConfig) string {
	sb := strings.Builder{}
	sb.WriteString("mode=")
	sb.WriteString(sub.Mode)
	sb.WriteString(", ")
	if strings.ToLower(sub.Mode) == "stream" {
		sb.WriteString("stream-mode=")
		sb.WriteString(sub.StreamMode)
		sb.WriteString(", ")
		if strings.ToLower(sub.StreamMode) == "sample" {
			sb.WriteString("sample-interval=")
			sb.WriteString(sub.SampleInterval.String())
			sb.WriteString(", ")
		}
	}
	sb.WriteString("encoding=")
	sb.WriteString(sub.Encoding)
	sb.WriteString(", ")
	if sub.Prefix != "" {
		sb.WriteString("prefix=")
		sb.WriteString(sub.Prefix)
		sb.WriteString(", ")
	}
	sb.WriteString("path(s)=")
	sb.WriteString(strings.Join(sub.Paths, ","))
	return sb.String()
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
	rootCmd.AddCommand(promptQuitCmd)
	rootCmd.RemoveCommand(promptModeCmd)
	shell := &cmdPrompt{
		RootCmd: rootCmd,
		GoPromptOptions: []goprompt.Option{
			goprompt.OptionTitle("gnmic-prompt"),
			goprompt.OptionPrefix("gnmic> "),
			goprompt.OptionHistory(promptHistory),
			goprompt.OptionMaxSuggestion(uint16(viper.GetUint("prompt-max-suggestions"))),
			goprompt.OptionPrefixTextColor(getColor("prompt-prefix-color")),
			goprompt.OptionPreviewSuggestionTextColor(goprompt.Cyan),
			goprompt.OptionSuggestionTextColor(goprompt.White),
			goprompt.OptionSuggestionBGColor(getColor("prompt-suggestions-bg-color")),
			goprompt.OptionSelectedSuggestionTextColor(goprompt.Black),
			goprompt.OptionSelectedSuggestionBGColor(goprompt.White),
			goprompt.OptionDescriptionTextColor(goprompt.LightGray),
			goprompt.OptionDescriptionBGColor(getColor("prompt-description-bg-color")),
			goprompt.OptionSelectedDescriptionTextColor(goprompt.Black),
			goprompt.OptionSelectedDescriptionBGColor(goprompt.White),
			goprompt.OptionScrollbarBGColor(goprompt.DarkGray),
			goprompt.OptionScrollbarThumbColor(goprompt.Blue),
			goprompt.OptionAddASCIICodeBind(
				// bind '?' character to show cmd args
				goprompt.ASCIICodeBind{
					ASCIICode: []byte{0x3f},
					Fn:        showCommandArguments,
				},
				// bind OS X Option+Left key binding
				goprompt.ASCIICodeBind{
					ASCIICode: []byte{0x1b, 0x62},
					Fn:        goprompt.GoLeftWord,
				},
				// bind OS X Option+Right key binding
				goprompt.ASCIICodeBind{
					ASCIICode: []byte{0x1b, 0x66},
					Fn:        goprompt.GoRightWord,
				},
			),
			goprompt.OptionAddKeyBind(
				// bind Linux CTRL+Left key binding
				goprompt.KeyBind{
					Key: goprompt.ControlLeft,
					Fn:  goprompt.GoLeftWord,
				},
				// bind Linux CTRL+Right key binding
				goprompt.KeyBind{
					Key: goprompt.ControlRight,
					Fn:  goprompt.GoRightWord,
				},
				// bind CTRL+Z key to delete path elements
				goprompt.KeyBind{
					Key: goprompt.ControlZ,
					Fn: func(buf *goprompt.Buffer) {
						// If the last word before the cursor does not contain a "/" return.
						// This is needed to avoid deleting down to a previous flag value
						if !strings.Contains(buf.Document().GetWordBeforeCursorWithSpace(), "/") {
							return
						}
						// Check if the last rune is a PathSeparator and is not the path root then delete it
						if buf.Document().GetCharRelativeToCursor(0) == os.PathSeparator && buf.Document().GetCharRelativeToCursor(-1) != ' ' {
							buf.DeleteBeforeCursor(1)
						}
						// Delete down until the next "/"
						buf.DeleteBeforeCursor(len([]rune(buf.Document().GetWordBeforeCursorUntilSeparator("/"))))
					},
				},
			),
			goprompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator),
			// goprompt.OptionCompletionOnDown(),
			goprompt.OptionShowCompletionAtStart(),
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
				err := co.RootCmd.Execute()
				if err == nil && in != "" {
					promptHistory = append(promptHistory, in)
				}
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
	command := co.RootCmd
	args := strings.Fields(doc.CurrentLine())
	if found, _, err := command.Find(args); err == nil {
		command = found
	}

	suggestions := make([]goprompt.Suggest, 0, 32)

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
	// add sub commands suggestions if they exist
	if command.HasAvailableSubCommands() {
		for _, c := range command.Commands() {
			if !c.Hidden {
				suggestions = append(suggestions, goprompt.Suggest{Text: c.Name(), Description: c.Short})
			}
		}
	}
	addFlags := func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		suggestions = append(suggestions, goprompt.Suggest{Text: "--" + flag.Name, Description: flag.Usage})
	}
	// load local flags
	command.LocalFlags().VisitAll(addFlags)
	if viper.GetBool("prompt-suggest-all-flags") {
		// load inherited flags
		command.InheritedFlags().VisitAll(addFlags)
	}

	return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
}

func resolveGlobs(globs []string) ([]string, error) {
	results := make([]string, 0, len(globs))
	for _, pattern := range globs {
		if strings.ContainsAny(pattern, `*?[`) {
			// is a glob pattern
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, err
			}
			results = append(results, matches...)
		} else {
			// is not a glob pattern ( file or dir )
			results = append(results, pattern)
		}
	}
	return results, nil
}
