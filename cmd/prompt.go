package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
	goprompt "github.com/c-bata/go-prompt"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc/grpclog"
)

var promptMode bool
var schemaTree *yang.Entry
var promptModeCmd = &cobra.Command{
	Use:   "prompt-mode",
	Short: "Run the interactive gnmic-prompt",
	RunE: func(cmd *cobra.Command, args []string) error {
		if promptMode {
			return fmt.Errorf("already entered to the prompt-mode")
		}
		ExecutePrompt(handleDynamicSuggestions)
		return nil
	},
}

func findMatchedXPATH(entry *yang.Entry, word string, cursor int) []goprompt.Suggest {
	suggestions := make([]goprompt.Suggest, 0, 4)
	cword := word[cursor:]
	for name, child := range entry.Dir {
		key := "/" + name
		if strings.HasPrefix(key, cword) {
			stext := fmt.Sprintf("%s%s", word[:cursor], key)
			suggestions = append(suggestions, goprompt.Suggest{Text: stext, Description: child.Description})
		} else if strings.HasPrefix(cword, key) {
			suggestions = append(suggestions, findMatchedXPATH(child, word, cursor+len(key))...)
		}
	}
	return suggestions
}

func handleDynamicSuggestions(annotation string, doc goprompt.Document) []goprompt.Suggest {
	switch annotation {
	case "XPATH":
		word := doc.GetWordBeforeCursor()
		suggestions := make([]goprompt.Suggest, 0, 16)
		for _, entry := range schemaTree.Dir {
			suggestions = append(suggestions, findMatchedXPATH(entry, word, 0)...)
		}
		return suggestions
	case "FILE":
	}
	return []goprompt.Suggest{}
}

func init() {
	rootCmd.AddCommand(promptModeCmd)
}

// promptRootCmd represents the base command when called without any subcommands
var promptRootCmd = &cobra.Command{
	Use:   "gnmic-prompt",
	Short: "start gnmi-prompt to run gNMI rpcs on the interacitve terminal",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		debug := viper.GetBool("debug")
		if viper.GetString("log-file") != "" {
			var err error
			f, err = os.OpenFile(viper.GetString("log-file"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				logger.Fatalf("error opening file: %v", err)
			}
		} else {
			if debug {
				viper.Set("log", true)
			}
			switch viper.GetBool("log") {
			case true:
				f = os.Stderr
			case false:
				f = myWriteCloser{ioutil.Discard}
			}
		}
		loggingFlags := log.LstdFlags | log.Lmicroseconds
		if debug {
			loggingFlags |= log.Llongfile
		}
		logger = log.New(f, "gnmic ", loggingFlags)
		if debug {
			grpclog.SetLogger(logger) //lint:ignore SA1019 see https://github.com/karimra/gnmic/issues/59
		}
		cfgFile := viper.ConfigFileUsed()
		if len(cfgFile) != 0 {
			logger.Printf("using config file %s", cfgFile)
			if debug {
				b, err := ioutil.ReadFile(cfgFile)
				if err != nil {
					logger.Printf("failed reading config file %s: %v", cfgFile, err)
					return
				}
				logger.Printf("config file:\n%s", string(b))
			}
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			for _, a := range args {
				if a == "prompt-mode" {
					return nil
				}
			}
			return fmt.Errorf("unknown command %v", args)
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if !viper.GetBool("log") || viper.GetString("log-file") != "" {
			f.Close()
		}
	},
}

var promptQuitCmd = &cobra.Command{
	Use:   "quit",
	Short: "quit the gnmic-prompt",
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(0)
	},
}

// ExecutePrompt load and run gnmic-prompt mode.
// This is called by main.main(). It only needs to happen once to the promptRootCmd.
func ExecutePrompt(dynamicSuggestionFunc func(annotation string, document goprompt.Document) []goprompt.Suggest) {
	cobra.OnInitialize(initConfig)
	promptRootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/gnmic.yaml)")
	promptRootCmd.PersistentFlags().StringSliceP("address", "a", []string{}, "comma separated gnmi targets addresses")
	promptRootCmd.PersistentFlags().StringP("username", "u", "", "username")
	promptRootCmd.PersistentFlags().StringP("password", "p", "", "password")
	promptRootCmd.PersistentFlags().StringP("port", "", defaultGrpcPort, "gRPC port")
	promptRootCmd.PersistentFlags().StringP("encoding", "e", "json", fmt.Sprintf("one of %+v. Case insensitive", encodings))
	promptRootCmd.PersistentFlags().BoolP("insecure", "", false, "insecure connection")
	promptRootCmd.PersistentFlags().StringP("tls-ca", "", "", "tls certificate authority")
	promptRootCmd.PersistentFlags().StringP("tls-cert", "", "", "tls certificate")
	promptRootCmd.PersistentFlags().StringP("tls-key", "", "", "tls key")
	promptRootCmd.PersistentFlags().DurationP("timeout", "", 10*time.Second, "grpc timeout, valid formats: 10s, 1m30s, 1h")
	promptRootCmd.PersistentFlags().BoolP("debug", "d", false, "debug mode")
	promptRootCmd.PersistentFlags().BoolP("skip-verify", "", false, "skip verify tls connection")
	promptRootCmd.PersistentFlags().BoolP("no-prefix", "", false, "do not add [ip:port] prefix to print output in case of multiple targets")
	promptRootCmd.PersistentFlags().BoolP("proxy-from-env", "", false, "use proxy from environment")
	promptRootCmd.PersistentFlags().StringP("format", "", "", "output format, one of: [protojson, prototext, json, event]")
	promptRootCmd.PersistentFlags().StringP("log-file", "", "", "log file path")
	promptRootCmd.PersistentFlags().BoolP("log", "", false, "show log messages in stderr")
	promptRootCmd.PersistentFlags().IntP("max-msg-size", "", msgSize, "max grpc msg size")
	promptRootCmd.PersistentFlags().StringP("prometheus-address", "", "", "prometheus server address")
	promptRootCmd.PersistentFlags().BoolP("print-request", "", false, "print request as well as the response(s)")

	viper.BindPFlag("address", promptRootCmd.PersistentFlags().Lookup("address"))
	viper.BindPFlag("username", promptRootCmd.PersistentFlags().Lookup("username"))
	viper.BindPFlag("password", promptRootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("port", promptRootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("encoding", promptRootCmd.PersistentFlags().Lookup("encoding"))
	viper.BindPFlag("insecure", promptRootCmd.PersistentFlags().Lookup("insecure"))
	viper.BindPFlag("tls-ca", promptRootCmd.PersistentFlags().Lookup("tls-ca"))
	viper.BindPFlag("tls-cert", promptRootCmd.PersistentFlags().Lookup("tls-cert"))
	viper.BindPFlag("tls-key", promptRootCmd.PersistentFlags().Lookup("tls-key"))
	viper.BindPFlag("timeout", promptRootCmd.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("debug", promptRootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("skip-verify", promptRootCmd.PersistentFlags().Lookup("skip-verify"))
	viper.BindPFlag("no-prefix", promptRootCmd.PersistentFlags().Lookup("no-prefix"))
	viper.BindPFlag("proxy-from-env", promptRootCmd.PersistentFlags().Lookup("proxy-from-env"))
	viper.BindPFlag("format", promptRootCmd.PersistentFlags().Lookup("format"))
	viper.BindPFlag("log-file", promptRootCmd.PersistentFlags().Lookup("log-file"))
	viper.BindPFlag("log", promptRootCmd.PersistentFlags().Lookup("log"))
	viper.BindPFlag("max-msg-size", promptRootCmd.PersistentFlags().Lookup("max-msg-size"))
	viper.BindPFlag("prometheus-address", promptRootCmd.PersistentFlags().Lookup("prometheus-address"))
	viper.BindPFlag("print-request", promptRootCmd.PersistentFlags().Lookup("print-request"))

	// Read preconfigured global options from arguments or config file.
	if err := promptRootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var err error
	schemaTree, err = loadSchemaZip()
	if err != nil {
		schemaTree = buildRootEntry()
	}

	// goprompt.OptionHistory()

	rootCmd.AddCommand(promptQuitCmd)
	shell := &cmdPrompt{
		RootCmd:                rootCmd,
		DynamicSuggestionsFunc: dynamicSuggestionFunc,
		ResetFlagsFlag:         true,
		GoPromptOptions: []goprompt.Option{
			goprompt.OptionTitle("gnmic-prompt"),
			goprompt.OptionPrefix("gnmic> "),
			goprompt.OptionMaxSuggestion(5),
			goprompt.OptionPrefixTextColor(prompt.Yellow),
		},
	}
	promptMode = true
	shell.Run()
}

// Reference: https://github.com/stromland/cobra-prompt

// cmdPrompt requires RootCmd to run
type cmdPrompt struct {
	// RootCmd is the start point, all its sub commands and flags will be available as suggestions
	RootCmd *cobra.Command

	// startup input arguments
	StartupArgs []string

	// GoPromptOptions is for customize go-prompt
	// see https://github.com/c-bata/go-prompt/blob/master/option.go
	GoPromptOptions []goprompt.Option

	// DynamicSuggestionsFunc will be executed if an command has CALLBACK_ANNOTATION as an annotation. If it's included
	// the value will be provided to the DynamicSuggestionsFunc function.
	DynamicSuggestionsFunc func(annotation string, document goprompt.Document) []goprompt.Suggest

	// ResetFlagsFlag will add a new persistent flag to RootCmd. This flags can be used to turn off flags value reset
	ResetFlagsFlag bool
}

// Run will automatically generate suggestions for all cobra commands and flags defined by RootCmd
// and execute the selected commands. Run will also reset all given flags by default, see ResetFlagsFlag
func (co cmdPrompt) Run() {
	co.prepare()
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

func (co cmdPrompt) prepare() {
	if co.ResetFlagsFlag {
		co.RootCmd.PersistentFlags().BoolP("flags-no-reset", "",
			false, "Flags will no longer reset to default value")
	}
}

func findSuggestions(co cmdPrompt, doc goprompt.Document) []goprompt.Suggest {
	command := co.RootCmd
	args := strings.Fields(doc.CurrentLine())

	if found, _, err := command.Find(args); err == nil {
		command = found
	}

	suggestions := make([]goprompt.Suggest, 0, 32)
	resetFlags, _ := command.Flags().GetBool("flags-no-reset")
	addFlags := func(flag *pflag.Flag) {
		if flag.Changed && !resetFlags {
			flag.Value.Set(flag.DefValue)
		}
		if flag.Hidden {
			return
		}
		if strings.HasPrefix(doc.GetWordBeforeCursor(), "--") {
			suggestions = append(suggestions, goprompt.Suggest{Text: "--" + flag.Name, Description: flag.Usage})
		} else if strings.HasPrefix(doc.GetWordBeforeCursor(), "-") && flag.Shorthand != "" {
			suggestions = append(suggestions, goprompt.Suggest{Text: "-" + flag.Shorthand, Description: flag.Usage})
		}
	}

	// load local flags of the command
	command.LocalFlags().VisitAll(addFlags)
	// parent flag is shown if run.
	// command.InheritedFlags().VisitAll(addFlags)

	if command.HasAvailableSubCommands() {
		for _, c := range command.Commands() {
			if !c.Hidden {
				suggestions = append(suggestions, goprompt.Suggest{Text: c.Name(), Description: c.Short})
			}
		}
	}

	// check flag annotation for the suggestion
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
	if co.DynamicSuggestionsFunc != nil && annotation != "" {
		suggestions = append(suggestions, co.DynamicSuggestionsFunc(annotation, doc)...)
	}
	return goprompt.FilterHasPrefix(suggestions, doc.GetWordBeforeCursor(), true)
}
