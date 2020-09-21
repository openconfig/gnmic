// Copyright © 2020 Karim Radhouani <medkarimrdi@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/gnxi/utils/xpath"
	"github.com/manifoldco/promptui"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var files []string
var excluded []string
var dirs []string
var pathType string
var module string
var printTypes bool
var search bool

// pathCmd represents the path command
var pathCmd = &cobra.Command{
	Use:     "path",
	Aliases: []string{"p"},
	Short:   "generate gnmi or xpath style from yang file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pathType != "xpath" && pathType != "gnmi" {
			err := fmt.Errorf("path-type must be one of 'xpath' or 'gnmi'")
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		for _, dirpath := range dirs {
			expanded, err := yang.PathsWithModules(dirpath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			yang.AddPath(expanded...)
		}

		ms := yang.NewModules()
		for _, name := range files {
			if err := ms.Read(name); err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
		}
		if errors := ms.Process(); len(errors) > 0 {
			for _, err := range errors {
				fmt.Fprintln(os.Stderr, err)
			}
			return fmt.Errorf("yang processing failed")
		}
		mods := map[string]*yang.Module{}
		names := []string{}

		for _, m := range ms.Modules {
			if mods[m.Name] == nil {
				mods[m.Name] = m
				names = append(names, m.Name)
			}
		}
		sort.Strings(names)
		entries := make([]*yang.Entry, len(names))
		for x, n := range names {
			skip := false
			for i := range excluded {
				if excluded[i] == n {
					skip = true
				}
			}
			if !skip {
				entries[x] = yang.ToEntry(mods[n])
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		out := make(chan string)
		defer close(out)
		paths := make([]string, 0)
		if search {
			go gather(ctx, out, &paths)
		} else {
			go printer(ctx, out)
		}
		collected := make([]*yang.Entry, 0, 256)
		for _, entry := range entries {
			collected = append(collected, collectSchemaNodes(entry, true)...)
		}
		for _, entry := range collected {
			out <- generatePath(entry, false)
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
					count := 0
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
					if result {
						count++
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
			index, selected, err := p.Run()
			if err != nil {
				return err
			}
			fmt.Println(selected)
			fmt.Println(generateEntryTyeInfo(collected[index]))
		}
		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		cmd.ResetFlags()
		initPathFlags(cmd)
	},
}

func init() {
	rootCmd.AddCommand(pathCmd)
	pathCmd.SilenceUsage = true
	initPathFlags(pathCmd)
}

// used to init or reset pathCmd flags for gnmic-prompt mode
func initPathFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&files, "file", "", []string{}, "yang files to get the paths")
	cmd.MarkFlagRequired("file")
	cmd.Flags().StringArrayVarP(&excluded, "exclude", "", []string{}, "yang files to be excluded from path generation")
	cmd.Flags().StringArrayVarP(&dirs, "dir", "", []string{}, "directories to search yang includes and imports")
	cmd.Flags().StringVarP(&pathType, "path-type", "", "xpath", "path type xpath or gnmi")
	cmd.Flags().StringVarP(&module, "module", "m", "", "module name")
	cmd.Flags().BoolVarP(&printTypes, "types", "", false, "print leaf type")
	cmd.Flags().BoolVarP(&search, "search", "", false, "search through path list")
	viper.BindPFlag("path-file", cmd.LocalFlags().Lookup("file"))
	viper.BindPFlag("path-dir", cmd.LocalFlags().Lookup("dir"))
	viper.BindPFlag("path-path-type", cmd.LocalFlags().Lookup("path-type"))
	viper.BindPFlag("path-module", cmd.LocalFlags().Lookup("module"))
	viper.BindPFlag("path-types", cmd.LocalFlags().Lookup("types"))
	viper.BindPFlag("path-search", cmd.LocalFlags().Lookup("search"))
	yang.Path = []string{}
}

func collectSchemaNodes(e *yang.Entry, leafOnly bool) []*yang.Entry {
	if e == nil {
		return []*yang.Entry{}
	}
	collected := make([]*yang.Entry, 0, 128)
	for _, child := range e.Dir {
		collected = append(collected,
			collectSchemaNodes(child, leafOnly)...)
	}
	if e.Parent != nil {
		switch {
		case e.Dir == nil && e.ListAttr != nil: // leaf-list
			fallthrough
		case e.Dir == nil: // leaf
			collected = append(collected, e)
		case e.ListAttr != nil: // list
			fallthrough
		default: // container
			if !leafOnly {
				collected = append(collected, e)
			}
		}
	}
	return collected
}

func generatePath(entry *yang.Entry, prefixTagging bool) string {
	path := ""
	for e := entry; e != nil && e.Parent != nil; e = e.Parent {
		elementName := e.Name
		if prefixTagging && e.Prefix != nil {
			elementName = e.Prefix.Name + ":" + elementName
		}
		if e.Key != "" {
			keylist := strings.Split(e.Key, " ")
			for _, k := range keylist {
				if prefixTagging && e.Prefix != nil {
					k = e.Prefix.Name + ":" + k
				}
				elementName = fmt.Sprintf("%s[%s=*]", elementName, k)
			}
		}
		path = fmt.Sprintf("/%s%s", elementName, path)
	}
	if pathType == "gnmi" {
		gnmiPath, err := xpath.ToGNMIPath(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "path: %s could not be changed to gnmi: %v\n", path, err)
		}
		path = gnmiPath.String()
	}
	if printTypes {
		path = fmt.Sprintf("%s (type=%s)", path, entry.Type.Name)
	}
	return path
}

func generateEntryTyeInfo(e *yang.Entry) string {
	if e == nil || e.Type == nil {
		return "unknown type"
	}
	t := e.Type
	return generateTypeInfo(t, true)
}

func generateTypeInfo(t *yang.YangType, prefixTagging bool) string {
	if t == nil {
		return ""
	}

	rstr := fmt.Sprintf("- type: %s", t.Kind)
	switch t.Kind {
	case yang.Ybits:
		nameMap := t.Bit.NameMap()
		bitlist := make([]string, 0, len(nameMap))
		for bitstr := range nameMap {
			bitlist = append(bitlist, bitstr)
		}
		rstr += fmt.Sprintf(" %v", bitlist)
	case yang.Yenum:
		nameMap := t.Enum.NameMap()
		enumlist := make([]string, 0, len(nameMap))
		for enumstr := range nameMap {
			enumlist = append(enumlist, enumstr)
		}
		rstr += fmt.Sprintf(" %v", enumlist)
	case yang.Yleafref:
		rstr += fmt.Sprintf(" %q", t.Path)
	case yang.Yidentityref:
		rstr += fmt.Sprintf(" %q", t.IdentityBase.Name)
		identities := make([]string, 0, 64)
		for i := range t.IdentityBase.Values {
			if prefixTagging {
				identities = append(identities, t.IdentityBase.Values[i].PrefixedName())
			} else {
				identities = append(identities, t.IdentityBase.Values[i].Name)
			}
		}
		rstr += fmt.Sprintf(" %v", identities)
	case yang.Yunion:
		unionlist := make([]string, 0, len(t.Type))
		for i := range t.Type {
			unionlist = append(unionlist, fmt.Sprintf("%s", t.Type[i].Name))
		}
		rstr += fmt.Sprintf(" %v", unionlist)
	default:
	}
	rstr += fmt.Sprintf("\n")
	if t.Base != nil {
		base := yang.Source(t.Base)
		if base != "unknown" {
			rstr += fmt.Sprintf("- typedef location: %s\n", base)
		}
	}

	if t.Kind.String() != t.Root.Name {
		rstr += fmt.Sprintf("- root-type: %s\n", t.Root.Name)
	}
	if t.Units != "" {
		rstr += fmt.Sprintf("- units=%s\n", t.Units)
	}
	if t.Default != "" {
		rstr += fmt.Sprintf("- default=%q\n", t.Default)
	}
	if t.FractionDigits != 0 {
		rstr += fmt.Sprintf("- fraction-digits=%d\n", t.FractionDigits)
	}
	if len(t.Length) > 0 {
		rstr += fmt.Sprintf("- length=%s\n", t.Length)
	}
	if t.Kind == yang.YinstanceIdentifier && !t.OptionalInstance {
		rstr += fmt.Sprintf("- required\n")
	}

	if len(t.Pattern) > 0 {
		rstr += fmt.Sprintf("- pattern=%s\n", strings.Join(t.Pattern, "|"))
	}
	b := yang.BaseTypedefs[t.Kind.String()].YangType
	if len(t.Range) > 0 && !t.Range.Equal(b.Range) {
		rstr += fmt.Sprintf("- range=%s\n", t.Range)
	}
	return rstr
}
