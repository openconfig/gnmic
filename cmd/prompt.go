package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	goprompt "github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/karimra/gnmic/types"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/nsf/termbox-go"
	"github.com/olekukonko/tablewriter"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

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

var targetListHeader = []string{
	"Name", "Address", "Username", "Password", "Insecure", "Skip Verify", "TLS CA", "TLS Certificate", "TLS Key"}

var subscriptionListHeader = []string{"Name", "Mode", "Prefix", "Paths", "Interval", "Encoding"}

func getColor(flagName string) goprompt.Color {
	switch flagName {
	case "prefix-color":
		if cgoprompt, ok := colorMapping[gApp.Config.LocalFlags.PromptPrefixColor]; ok {
			return cgoprompt
		}
	case "suggestions-bg-color":
		if cgoprompt, ok := colorMapping[gApp.Config.LocalFlags.PromptSuggestionsBGColor]; ok {
			return cgoprompt
		}
	case "description-bg-color":
		if cgoprompt, ok := colorMapping[gApp.Config.LocalFlags.PromptDescriptionBGColor]; ok {
			return cgoprompt
		}
	}
	defColor := "yellow"
	promptModeCmd.Flags().VisitAll(
		func(f *pflag.Flag) {
			if f.Name == flagName {
				defColor = f.DefValue
				return
			}
		},
	)
	return colorMapping[defColor]
}

var promptModeCmd *cobra.Command

func newPromptCmd() *cobra.Command {
	promptModeCmd = &cobra.Command{
		Use:     "prompt",
		Short:   "enter the interactive gnmic prompt mode",
		PreRunE: gApp.PromptPreRunE,
		RunE:    gApp.PromptRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			//initPromptFlags(cmd)
		},
		SilenceUsage: true,
	}
	gApp.InitPromptFlags(promptModeCmd)
	return promptModeCmd
}

var promptQuitCmd = &cobra.Command{
	Use:   "quit",
	Short: "quit the gnmic-prompt",
	Run: func(cmd *cobra.Command, args []string) {
		// cancel gctx
		gApp.Cfn()
		// save history
		home, err := homedir.Dir()
		if err != nil {
			os.Exit(0)
		}
		f, err := os.Create(home + "/.gnmic.history")
		if err != nil {
			os.Exit(0)
		}
		l := len(gApp.PromptHistory)
		if l > 128 {
			gApp.PromptHistory = gApp.PromptHistory[l-128:]
		}
		for i := range gApp.PromptHistory {
			f.WriteString(gApp.PromptHistory[i] + "\n")
		}
		f.Close()
		os.Exit(0)
	},
}

var targetCmd = &cobra.Command{
	Use:   "target",
	Short: "manipulate configured targets",
}

var targetListCmd = &cobra.Command{
	Use:   "list",
	Short: "list configured targets",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetsConfig, err := gApp.Config.GetTargets()
		if err != nil {
			return err
		}
		tabData := targetTable(targetsConfig, true)
		renderTable(tabData, targetListHeader)
		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		name = ""
	},
}

var targetShowCmd = &cobra.Command{
	Use:   "show",
	Short: "show a target details",
	Annotations: map[string]string{
		"--name": "TARGET",
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			fmt.Println("provide a target name with --name")
			return nil
		}
		targetsConfig, err := gApp.Config.GetTargets()
		if err != nil {
			return err
		}
		if tc, ok := targetsConfig[name]; ok {
			tabData := targetTable(map[string]*types.TargetConfig{name: tc}, false)
			renderTable(tabData, []string{"Param", "Value"})
			return nil
		}
		return errors.New("unknown target")
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		name = ""
	},
}

var subscriptionCmd = &cobra.Command{
	Use:   "subscription",
	Short: "manipulate configured subscriptions",
}

var subscriptionListCmd = &cobra.Command{
	Use:   "list",
	Short: "list configured subscriptions",
	RunE: func(cmd *cobra.Command, args []string) error {
		subs, err := gApp.Config.GetSubscriptions(nil)
		if err != nil {
			return err
		}
		tabData := subscriptionTable(subs, true)
		renderTable(tabData, subscriptionListHeader)
		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		name = ""
	},
}

var subscriptionShowCmd = &cobra.Command{
	Use:   "show",
	Short: "show a subscription details",
	Annotations: map[string]string{
		"--name": "SUBSCRIPTION",
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			fmt.Println("provide a subscription name with --name")
			return nil
		}
		subs, err := gApp.Config.GetSubscriptions(nil)
		if err != nil {
			return err
		}
		if s, ok := subs[name]; ok {
			tabData := subscriptionTable(map[string]*types.SubscriptionConfig{name: s}, false)
			renderTable(tabData, []string{"Param", "Value"})
			return nil
		}
		return errors.New("unknown subscription")
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		name = ""
	},
}

var outputCmd = &cobra.Command{
	Use:   "output",
	Short: "manipulate configured outputs",
}

var outputListCmd = &cobra.Command{
	Use:   "list",
	Short: "list configured outputs",
	RunE: func(cmd *cobra.Command, args []string) error {
		tabData := gApp.Config.GetOutputsConfigs()
		renderTable(tabData, []string{"Name", "Config"})
		return nil
	},
}

func renderTable(tabData [][]string, header []string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)
	table.AppendBulk(tabData)
	table.Render()
}

func targetTable(targetConfigs map[string]*types.TargetConfig, list bool) [][]string {
	if list {
		tabData := make([][]string, 0)
		for _, tc := range targetConfigs {
			tabData = append(tabData, []string{
				tc.Name,
				tc.Address,
				tc.UsernameString(),
				tc.PasswordString(),
				tc.InsecureString(),
				tc.SkipVerifyString(),
				tc.TLSCAString(),
				tc.TLSCertString(),
				tc.TLSKeyString(),
			})
		}
		sort.Slice(tabData, func(i, j int) bool {
			return tabData[i][0] < tabData[j][0]
		})
		return tabData
	}
	if len(targetConfigs) > 1 {
		gApp.Logger.Printf("cannot show multiple targets")
		return nil
	}
	for _, tc := range targetConfigs {
		tabData := make([][]string, 0, 16)
		tabData = append(tabData, []string{"Name", tc.Name})
		tabData = append(tabData, []string{"Address", tc.Address})
		tabData = append(tabData, []string{"Username", tc.UsernameString()})
		tabData = append(tabData, []string{"Password", tc.PasswordString()})
		tabData = append(tabData, []string{"Insecure", tc.InsecureString()})
		tabData = append(tabData, []string{"Skip Verify", tc.SkipVerifyString()})
		tabData = append(tabData, []string{"TLS CA", tc.TLSCAString()})
		tabData = append(tabData, []string{"TLS Certificate", tc.TLSCertString()})
		tabData = append(tabData, []string{"TLS Key", tc.TLSKeyString()})
		tabData = append(tabData, []string{"TLS Min Version", tc.TLSMinVersion})
		tabData = append(tabData, []string{"TLS Max Version", tc.TLSMaxVersion})
		tabData = append(tabData, []string{"TLS Version", tc.TLSVersion})
		tabData = append(tabData, []string{"Subscriptions", strings.Join(tc.Subscriptions, "\n")})
		tabData = append(tabData, []string{"Outputs", strings.Join(tc.Outputs, "\n")})
		tabData = append(tabData, []string{"Buffer Size", tc.BufferSizeString()})
		tabData = append(tabData, []string{"Retry Timer", tc.RetryTimer.String()})
		return tabData
	}
	return [][]string{}
}

func subscriptionTable(scs map[string]*types.SubscriptionConfig, list bool) [][]string {
	if list {
		tabData := make([][]string, 0, len(scs))
		for _, sub := range scs {
			tabData = append(tabData, []string{
				sub.Name,
				sub.ModeString(),
				sub.PrefixString(),
				sub.PathsString(),
				sub.SampleIntervalString(),
				sub.Encoding,
			})
		}
		sort.Slice(tabData, func(i, j int) bool {
			return tabData[i][0] < tabData[j][0]
		})
		return tabData
	}
	if len(scs) > 1 {
		gApp.Logger.Printf("cannot show multiple subscriptions")
		return nil
	}
	for _, sub := range scs {
		tabData := make([][]string, 0, 8)
		tabData = append(tabData, []string{"Name", sub.Name})
		tabData = append(tabData, []string{"Mode", sub.ModeString()})
		tabData = append(tabData, []string{"Prefix", sub.PrefixString()})
		tabData = append(tabData, []string{"Paths", sub.PathsString()})
		tabData = append(tabData, []string{"Sample Interval", sub.SampleIntervalString()})
		tabData = append(tabData, []string{"Encoding", sub.Encoding})
		tabData = append(tabData, []string{"Qos", sub.QosString()})
		tabData = append(tabData, []string{"Heartbeat Interval", sub.HeartbeatIntervalString()})
		return tabData
	}
	return [][]string{}
}

var name string

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

	prependOrigin := gApp.Config.LocalFlags.PromptSuggestWithOrigin && !prefixPresent
	for name, child := range entry.Dir {
		if child.IsCase() || child.IsChoice() {
			for _, gchild := range child.Dir {
				suggestions = append(suggestions, findMatchedXPATH(gchild, input, prefixPresent)...)
			}
			continue
		}
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
	if gApp.Config.LocalFlags.PromptDescriptionWithTypes {
		n, _ := sb.WriteString(getEntryType(entry))
		if n > 0 {
			sb.WriteString(", ")
		}
	}
	if gApp.Config.LocalFlags.PromptDescriptionWithPrefix {
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
	for name, child := range entry.Dir {
		pathelem := "/" + name
		if strings.HasPrefix(pathelem, input) {
			schemaNodes = append(schemaNodes, child)
			if child.Key != "" { // list

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
				for _, entry := range gApp.SchemaTree.Dir {
					entries = append(entries, findMatchedSchema(entry, line)...)
				}
				// generate suggestions from matching entries
				for _, entry := range entries {
					suggestions = append(suggestions, findMatchedXPATH(entry, word, true)...)
				}
			}
		} else {
			// generate suggestions from yang schema
			for _, entry := range gApp.SchemaTree.Dir {
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
		for _, entry := range gApp.SchemaTree.Dir {
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
		suggestions := make([]goprompt.Suggest, 0, len(gApp.SchemaTree.Dir))
		for name, dir := range gApp.SchemaTree.Dir {
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
		subs := gApp.Config.GetSubscriptionsFromFile()
		suggestions := make([]goprompt.Suggest, 0, len(subs))
		for _, sub := range subs {
			suggestions = append(suggestions, goprompt.Suggest{Text: sub.Name, Description: subscriptionDescription(sub)})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	case "TARGET":
		targetsConfig := gApp.Config.TargetsList()
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
		outputGroups := gApp.Config.GetOutputsSuggestions()
		suggestions := make([]goprompt.Suggest, 0, len(outputGroups))
		for _, sugg := range outputGroups {
			suggestions = append(suggestions, goprompt.Suggest{Text: sugg.Name, Description: strings.Join(sugg.Types, ", ")})
		}
		return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
	}
	return []goprompt.Suggest{}
}

func subscriptionDescription(sub *types.SubscriptionConfig) string {
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
	command := gApp.RootCmd
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
		gApp.Logger.Fatalf("%v", err)
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
	initPromptCmds()
	shell := &cmdPrompt{
		RootCmd: gApp.RootCmd,
		GoPromptOptions: []goprompt.Option{
			goprompt.OptionTitle("gnmic-prompt"),
			goprompt.OptionPrefix("gnmic> "),
			goprompt.OptionHistory(gApp.PromptHistory),
			goprompt.OptionMaxSuggestion(gApp.Config.LocalFlags.PromptMaxSuggestions),
			goprompt.OptionPrefixTextColor(getColor("prefix-color")),
			goprompt.OptionPreviewSuggestionTextColor(goprompt.Cyan),
			goprompt.OptionSuggestionTextColor(goprompt.White),
			goprompt.OptionSuggestionBGColor(getColor("suggestions-bg-color")),
			goprompt.OptionSelectedSuggestionTextColor(goprompt.Black),
			goprompt.OptionSelectedSuggestionBGColor(goprompt.White),
			goprompt.OptionDescriptionTextColor(goprompt.LightGray),
			goprompt.OptionDescriptionBGColor(getColor("description-bg-color")),
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

func initPromptCmds() {
	gApp.RootCmd.AddCommand(promptQuitCmd)
	gApp.RootCmd.AddCommand(targetCmd)
	gApp.RootCmd.AddCommand(subscriptionCmd)
	gApp.RootCmd.AddCommand(outputCmd)

	targetCmd.AddCommand(targetListCmd)
	targetCmd.AddCommand(targetShowCmd)
	targetShowCmd.Flags().StringVarP(&name, "name", "", "", "target name")

	subscriptionCmd.AddCommand(subscriptionListCmd)
	subscriptionCmd.AddCommand(subscriptionShowCmd)
	subscriptionShowCmd.Flags().StringVarP(&name, "name", "", "", "subscription name")

	outputCmd.AddCommand(outputListCmd)

	gApp.RootCmd.RemoveCommand(promptModeCmd)
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
			promptArgs, err := parsePromptArgs(in)
			if err != nil {
				fmt.Fprint(os.Stderr, err)
				return
			}
			os.Args = append([]string{os.Args[0]}, promptArgs...)
			if len(promptArgs) > 0 {
				err := co.RootCmd.Execute()
				if err == nil && in != "" {
					gApp.PromptHistory = append(gApp.PromptHistory, in)
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

func parsePromptArgs(in string) ([]string, error) {
	var m = []string{}
	var s string

	// space suffix ensures the last string is appended
	in = strings.TrimSpace(in) + " "

	lastQuote := rune(0)
	isSpace := false
	for _, c := range in {
		switch {
		// ending a quoted item, break out, skip this character and reset lastQuote
		case c == lastQuote:
			lastQuote = rune(0)

		// in a quoted item, include this character
		case lastQuote != rune(0):
			s += string(c)

		// starting a quoted item, set lastQuote
		case unicode.In(c, unicode.Quotation_Mark):
			isSpace = false
			lastQuote = c

		// a space, append the string to the list
		// if it was not already added (previous char was a space)
		// and reset string s
		case unicode.IsSpace(c):
			if isSpace {
				continue
			}
			isSpace = true
			m = append(m, s)
			s = ""
		// add the char to the string
		default:
			isSpace = false
			s += string(c)
		}
	}

	if lastQuote != rune(0) {
		return nil, fmt.Errorf("quotes not closed")
	}

	return m, nil
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
	if gApp.Config.LocalFlags.PromptSuggestAllFlags {
		// load inherited flags
		command.InheritedFlags().VisitAll(addFlags)
	}

	return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
}
