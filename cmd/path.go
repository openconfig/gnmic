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
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/google/gnxi/utils/xpath"
	"github.com/manifoldco/promptui"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygen"
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
var generateSchema bool
var prefixRepresented bool

// pathCmd represents the path command
var pathCmd = &cobra.Command{
	Use:   "path",
	Short: "generate gnmi or xpath style from yang file",
	Annotations: map[string]string{
		"--file": "YANG",
		"--dir":  "DIR",
	},
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
		// Keep track of the top level modules we read in.
		// Those are the only modules we want to print below.
		mods := map[string]*yang.Module{}
		var names []string

		for _, m := range ms.Modules {
			if mods[m.Name] == nil {
				mods[m.Name] = m
				names = append(names, m.Name)
			}
		}
		sort.Strings(names)
		entries := make([]*yang.Entry, len(names))
		for x, n := range names {
			entries[x] = yang.ToEntry(mods[n])
		}

		root := buildRootEntry()
		for _, entry := range entries {
			skip := false
			for i := range excluded {
				if entry.Name == excluded[i] {
					skip = true
				}
			}
			if !skip {
				updateAnnotation(entry)
				root.Dir[entry.Name] = entry
			}
		}
		if generateSchema {
			if err := writeSchemaZip(root); err != nil {
				return err
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
		for _, entry := range root.Dir {
			collected = append(collected, collectSchemaNodes(entry, true)...)
		}
		for _, entry := range collected {
			out <- generatePath(entry, prefixRepresented)
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
			fmt.Println(generateTypeInfo(collected[index]))
		}
		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		cmd.ResetFlags()
		initPathFlags(cmd)
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(pathCmd)
	initPathFlags(pathCmd)
}

// used to init or reset pathCmd flags for gnmic-prompt mode
func initPathFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&files, "file", "", []string{}, "yang files to get the paths")
	cmd.MarkFlagRequired("file")
	cmd.Flags().StringArrayVarP(&excluded, "exclude", "", []string{}, "yang modules to be excluded from path generation")
	cmd.Flags().StringArrayVarP(&dirs, "dir", "", []string{}, "directories to search yang includes and imports")
	cmd.Flags().StringVarP(&pathType, "path-type", "", "xpath", "path type xpath or gnmi")
	cmd.Flags().StringVarP(&module, "module", "m", "", "module name")
	cmd.Flags().BoolVarP(&prefixRepresented, "prefix-represented", "", false, "enable the yang prefix of the paths")
	cmd.Flags().BoolVarP(&printTypes, "types", "", false, "print leaf type")
	cmd.Flags().BoolVarP(&search, "search", "", false, "search through path list")
	cmd.Flags().BoolVarP(&generateSchema, "generate-schema", "g", false, "generate schema ($HOME/.gnmic.schema) for tab compeletion")
	viper.BindPFlag("path-file", cmd.LocalFlags().Lookup("file"))
	viper.BindPFlag("path-exclude", cmd.LocalFlags().Lookup("exclude"))
	viper.BindPFlag("path-dir", cmd.LocalFlags().Lookup("dir"))
	viper.BindPFlag("path-path-type", cmd.LocalFlags().Lookup("path-type"))
	viper.BindPFlag("path-module", cmd.LocalFlags().Lookup("module"))
	viper.BindPFlag("path-types", cmd.LocalFlags().Lookup("types"))
	viper.BindPFlag("path-search", cmd.LocalFlags().Lookup("search"))
	viper.BindPFlag("path-generate-schema", cmd.LocalFlags().Lookup("generate-schema"))
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

func generateTypeInfo(e *yang.Entry) string {
	if e == nil || e.Type == nil {
		return "unknown type"
	}
	t := e.Type
	rstr := fmt.Sprintf("- type: %s", t.Kind)
	switch t.Kind {
	case yang.Ybits:
		data := getAnnotation(e, "bits")
		if data != nil {
			rstr += fmt.Sprintf(" %v", data)
		}
	case yang.Yenum:
		data := getAnnotation(e, "enum")
		if data != nil {
			rstr += fmt.Sprintf(" %v", data)
		}
	case yang.Yleafref:
		rstr += fmt.Sprintf(" %q", t.Path)
	case yang.Yidentityref:
		rstr += fmt.Sprintf(" %q", t.IdentityBase.Name)
		if prefixRepresented {
			data := getAnnotation(e, "prefix-qualified-identities")
			if data != nil {
				rstr += fmt.Sprintf(" %v", data)
			}
		} else {
			identities := make([]string, 0, 64)
			for i := range t.IdentityBase.Values {
				identities = append(identities, t.IdentityBase.Values[i].Name)
			}
			rstr += fmt.Sprintf(" %v", identities)
		}

	case yang.Yunion:
		unionlist := make([]string, 0, len(t.Type))
		for i := range t.Type {
			unionlist = append(unionlist, fmt.Sprintf("%s", t.Type[i].Name))
		}
		rstr += fmt.Sprintf(" %v", unionlist)
	default:
	}
	rstr += fmt.Sprintf("\n")

	if t.Root != nil {
		data := getAnnotation(e, "root.type")
		if data != nil && t.Kind.String() != data.(string) {
			rstr += fmt.Sprintf("- root.type: %v\n", data)
		}
	}
	if t.Units != "" {
		rstr += fmt.Sprintf("- units: %s\n", t.Units)
	}
	if t.Default != "" {
		rstr += fmt.Sprintf("- default: %q\n", t.Default)
	}
	if t.FractionDigits != 0 {
		rstr += fmt.Sprintf("- fraction-digits: %d\n", t.FractionDigits)
	}
	if len(t.Length) > 0 {
		rstr += fmt.Sprintf("- length: %s\n", t.Length)
	}
	if t.Kind == yang.YinstanceIdentifier && !t.OptionalInstance {
		rstr += fmt.Sprintf("- required\n")
	}

	if len(t.Pattern) > 0 {
		rstr += fmt.Sprintf("- pattern: %s\n", strings.Join(t.Pattern, "|"))
	}
	b := yang.BaseTypedefs[t.Kind.String()].YangType
	if len(t.Range) > 0 && !t.Range.Equal(b.Range) {
		rstr += fmt.Sprintf("- range: %s\n", t.Range)
	}
	return rstr
}

func getAnnotation(entry *yang.Entry, name string) interface{} {
	if entry.Annotation != nil {
		data, ok := entry.Annotation[name]
		if ok {
			return data
		}
	}
	return nil
}

// updateAnnotation updates the schema info before enconding.
func updateAnnotation(entry *yang.Entry) {
	for _, child := range entry.Dir {
		updateAnnotation(child)
		child.Annotation = map[string]interface{}{}
		t := child.Type
		if t == nil {
			continue
		}

		switch t.Kind {
		case yang.Ybits:
			nameMap := t.Bit.NameMap()
			bits := make([]string, 0, len(nameMap))
			for bitstr := range nameMap {
				bits = append(bits, bitstr)
			}
			child.Annotation["bits"] = bits
		case yang.Yenum:
			nameMap := t.Enum.NameMap()
			enum := make([]string, 0, len(nameMap))
			for enumstr := range nameMap {
				enum = append(enum, enumstr)
			}
			child.Annotation["enum"] = enum
		case yang.Yidentityref:
			identities := make([]string, 0, len(t.IdentityBase.Values))
			for i := range t.IdentityBase.Values {
				identities = append(identities, t.IdentityBase.Values[i].PrefixedName())
			}
			child.Annotation["prefix-qualified-identities"] = identities
		}
		if t.Root != nil {
			child.Annotation["root.type"] = t.Root.Name
		}
	}
}

func buildRootEntry() *yang.Entry {
	rootEntry := &yang.Entry{
		Dir:        map[string]*yang.Entry{},
		Annotation: map[string]interface{}{},
	}
	rootEntry.Name = "root"
	rootEntry.Annotation["schemapath"] = "/"
	rootEntry.Kind = yang.DirectoryEntry
	// Always annotate the root as a fake root, so that it is not treated
	// as a path element in ytypes.
	rootEntry.Annotation["root"] = true
	return rootEntry
}

func writeSchemaZip(root *yang.Entry) error {
	j, err := json.MarshalIndent(root, "", strings.Repeat(" ", 4))
	if err != nil {
		return fmt.Errorf("JSON marshalling error: %v", err)
	}
	if len(j) == 0 {
		return nil
	}
	// fmt.Println(string(j))
	gotGzip, err := ygen.WriteGzippedByteSlice(j)
	if err != nil {
		return err
	}
	home, err := homedir.Dir()
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(home+"/.gnmic.schema", gotGzip, 0600)
	return err
}

func fixSchema(entry *yang.Entry) {
	for _, child := range entry.Dir {
		if child.Parent == nil {
			child.Parent = entry
		}
		fixSchema(child)
	}
}

func loadSchemaZip() (*yang.Entry, error) {
	home, err := homedir.Dir()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(home + "/.gnmic.schema")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// gotGzip, err := ioutil.ReadAll(f)
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	s, err := ioutil.ReadAll(gzr)
	if err != nil {
		return nil, err
	}
	root := &yang.Entry{}
	if err := json.Unmarshal(s, &root); err != nil {
		return nil, err
	}
	for _, eachModuleTop := range root.Dir {
		fixSchema(eachModuleTop)
	}
	return root, err
}
