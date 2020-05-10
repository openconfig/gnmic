package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/gnxi/utils/xpath"
	"github.com/manifoldco/promptui"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var file string
var pathType string
var module string
var types bool
var search bool

// pathCmd represents the path command
var pathCmd = &cobra.Command{
	Use:     "path",
	Aliases: []string{"p"},
	Short:   "generate gnmi or xpath style from yang file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pathType != "xpath" && pathType != "gnmi" {
			fmt.Println("path type must be one of 'xpath' or 'gnmi'")
			return nil
		}
		ms := yang.NewModules()

		if err := ms.Read(file); err != nil {
			return err
		}

		mod, ok := ms.Modules[module]
		if !ok {
			return fmt.Errorf("module %s not found", module)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		out := make(chan string, 0)
		defer close(out)
		paths := make([]string, 0)
		if search {
			go gather(ctx, out, &paths)
		} else {
			go printer(ctx, out)
		}
		for _, c := range mod.Container {
			addContainerToPath("", c, out)
		}
		if search {
			p := promptui.Select{
				Label:        "select path",
				Items:        paths,
				Size:         10,
				Stdout:       os.Stdout,
				HideSelected: true,
				Searcher: func(input string, index int) bool {
					kws := strings.Split(input, " ")
					result := true
					for _, kw := range kws {
						if strings.HasPrefix(kw, "!") {
							kw = strings.TrimLeft(kw, "!")
							if kw == "" {
								continue
							}
							result = result && !strings.Contains(paths[index], kw)
						} else {
							result = result && strings.Contains(paths[index], kw)
						}
					}
					return result
				},
				Keys: &promptui.SelectKeys{
					Prev:     promptui.Key{Code: promptui.KeyPrev, Display: promptui.KeyPrevDisplay},
					Next:     promptui.Key{Code: promptui.KeyNext, Display: promptui.KeyNextDisplay},
					PageUp:   promptui.Key{Code: promptui.KeyBackward, Display: promptui.KeyBackwardDisplay},
					PageDown: promptui.Key{Code: promptui.KeyForward, Display: promptui.KeyForwardDisplay},
					Search:   promptui.Key{Code: ':', Display: ":"},
				},
			}
			_, selected, err := p.Run()
			if err != nil {
				return err
			}
			fmt.Println(selected)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pathCmd)
	pathCmd.Flags().StringVarP(&file, "file", "", "", "yang file")
	pathCmd.Flags().StringVarP(&pathType, "path-type", "", "xpath", "path type xpath or gnmi")
	pathCmd.Flags().StringVarP(&module, "module", "m", "nokia-state", "module name")
	pathCmd.Flags().BoolVarP(&types, "types", "", false, "print leaf type")
	pathCmd.Flags().BoolVarP(&search, "search", "", false, "search through path list")
	viper.BindPFlag("file", pathCmd.Flags().Lookup("file"))
	pathCmd.SilenceUsage = true
}

func addContainerToPath(prefix string, container *yang.Container, out chan string) {
	elementName := fmt.Sprintf("%s/%s", prefix, container.Name)
	for _, c := range container.Container {
		addContainerToPath(elementName, c, out)
	}
	for _, ls := range container.List {
		addListToPath(elementName, ls, out)
	}
	for _, lf := range container.Leaf {
		path := fmt.Sprintf("%s/%s", elementName, lf.Name)
		if pathType == "gnmi" {
			gnmiPath, err := xpath.ToGNMIPath(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "path: %s could not be changed to gnmi: %v\n", path, err)
				continue
			}
			path = gnmiPath.String()
		}
		if types {
			path = fmt.Sprintf("%s (type=%v)", path, lf.Type.Name)
		}
		out <- path
	}
}
func addListToPath(prefix string, ls *yang.List, out chan string) {
	keys := strings.Split(ls.Key.Name, " ")
	keyElem := ls.Name
	for _, k := range keys {
		keyElem += fmt.Sprintf("[%s=*]", k)
	}
	elementName := fmt.Sprintf("%s/%s", prefix, keyElem)
	for _, c := range ls.Container {
		addContainerToPath(elementName, c, out)
	}
	for _, lls := range ls.List {
		addListToPath(elementName, lls, out)
	}
	for _, ch := range ls.Choice {
		for _, ca := range ch.Case {
			addCaseToPath(elementName, ca, out)
		}
	}
	for _, lf := range ls.Leaf {
		if lf.Name != ls.Key.Name {
			path := fmt.Sprintf("%s/%s", prefix, lf.Name)
			if pathType == "gnmi" {
				gnmiPath, err := xpath.ToGNMIPath(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "path: %s could not be changed to gnmi: %v\n", path, err)
					continue
				}
				path = gnmiPath.String()
			}
			if types {
				path = fmt.Sprintf("%s (type=%v)", path, lf.Type.Name)
			}
			out <- path
		}
	}
}
func addCaseToPath(prefix string, ca *yang.Case, out chan string) {
	for _, cont := range ca.Container {
		addContainerToPath(prefix, cont, out)
	}
	for _, ls := range ca.List {
		addListToPath(prefix, ls, out)
	}
	for _, lf := range ca.Leaf {
		path := fmt.Sprintf("%s/%s", prefix, lf.Name)
		if pathType == "gnmi" {
			gnmiPath, err := xpath.ToGNMIPath(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "path: %s could not be changed to gnmi: %v\n", path, err)
				continue
			}
			path = gnmiPath.String()
		}
		if types {
			path = fmt.Sprintf("%s (type=%v)", path, lf.Type.Name)
		}
		out <- path
	}
}
