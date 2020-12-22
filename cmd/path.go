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
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/gnxi/utils/xpath"
	"github.com/manifoldco/promptui"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func pathCmdRun(d, f, e []string) error {
	err := generateYangSchema(d, f, e)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan string)
	defer close(out)
	paths := make([]string, 0)
	if cli.config.LocalFlags.PathSearch {
		go gather(ctx, out, &paths)
	} else {
		go printer(ctx, out)
	}
	collected := make([]*yang.Entry, 0, 256)
	for _, entry := range cli.schemaTree.Dir {
		collected = append(collected, collectSchemaNodes(entry, true)...)
	}
	for _, entry := range collected {
		out <- generatePath(entry, cli.config.LocalFlags.PathWithPrefix)
	}

	if cli.config.LocalFlags.PathSearch {
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
		index, selected, err := p.Run()
		if err != nil {
			return err
		}
		fmt.Println(selected)
		fmt.Println(generateTypeInfo(collected[index]))
	}
	return nil
}

func generateYangSchema(d, f, e []string) error {
	if len(f) == 0 {
		return nil
	}

	ms := yang.NewModules()
	for _, name := range f {
		if err := ms.Read(name); err != nil {
			return err
		}
	}
	if errors := ms.Process(); len(errors) > 0 {
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "yang processing error: %v\n", e)
		}
		return fmt.Errorf("yang processing failed with %d errors", len(errors))
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

	cli.schemaTree = buildRootEntry()
	for _, entry := range entries {
		skip := false
		for i := range e {
			if entry.Name == e[i] {
				skip = true
			}
		}
		if !skip {
			updateAnnotation(entry)
			cli.schemaTree.Dir[entry.Name] = entry
		}
	}
	return nil
}

// pathCmd represents the path command
func newPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "generate gnmi or xpath style from yang file",
		Annotations: map[string]string{
			"--file": "YANG",
			"--dir":  "DIR",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			cli.config.SetLocalFlagsFromFile(cmd)
			if cli.config.LocalFlags.PathPathType != "xpath" && cli.config.LocalFlags.PathPathType != "gnmi" {
				return fmt.Errorf("path-type must be one of 'xpath' or 'gnmi'")
			}
			cli.config.LocalFlags.PathDir = sanitizeArrayFlagValue(cli.config.LocalFlags.PathDir)
			cli.config.LocalFlags.PathFile = sanitizeArrayFlagValue(cli.config.LocalFlags.PathFile)
			cli.config.LocalFlags.PathExclude = sanitizeArrayFlagValue(cli.config.LocalFlags.PathExclude)

			var err error
			cli.config.LocalFlags.PathDir, err = resolveGlobs(cli.config.LocalFlags.PathDir)
			if err != nil {
				return err
			}
			cli.config.LocalFlags.PathFile, err = resolveGlobs(cli.config.LocalFlags.PathFile)
			if err != nil {
				return err
			}
			for _, dirpath := range cli.config.LocalFlags.PathDir {
				expanded, err := yang.PathsWithModules(dirpath)
				if err != nil {
					return err
				}
				if cli.config.Globals.Debug {
					for _, fdir := range expanded {
						cli.logger.Printf("adding %s to YANG paths", fdir)
					}
				}
				yang.AddPath(expanded...)
			}
			yfiles, err := findYangFiles(cli.config.LocalFlags.PathFile)
			if err != nil {
				return err
			}
			cli.config.LocalFlags.PathFile = make([]string, 0, len(yfiles))
			cli.config.LocalFlags.PathFile = append(cli.config.LocalFlags.PathFile, yfiles...)
			if cli.config.Globals.Debug {
				for _, file := range cli.config.LocalFlags.PathFile {
					cli.logger.Printf("loading %s file", file)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return pathCmdRun(cli.config.LocalFlags.PathDir, cli.config.LocalFlags.PathFile, cli.config.LocalFlags.PathExclude)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initPathFlags(cmd)
		},
		SilenceUsage: true,
	}
	initPathFlags(cmd)
	return cmd
}

// used to init or reset pathCmd flags for gnmic-prompt mode
func initPathFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&cli.config.LocalFlags.PathFile, "file", "", []string{}, "yang files to get the paths")
	cmd.MarkFlagRequired("file")
	cmd.Flags().StringArrayVarP(&cli.config.LocalFlags.PathExclude, "exclude", "", []string{}, "yang modules to be excluded from path generation")
	cmd.Flags().StringArrayVarP(&cli.config.LocalFlags.PathDir, "dir", "", []string{}, "directories to search yang includes and imports")
	cmd.Flags().StringVarP(&cli.config.LocalFlags.PathPathType, "path-type", "", "xpath", "path type xpath or gnmi")
	cmd.Flags().StringVarP(&cli.config.LocalFlags.PathModule, "module", "m", "", "module name")
	cmd.Flags().BoolVarP(&cli.config.LocalFlags.PathWithPrefix, "with-prefix", "", false, "include module/submodule prefix in path elements")
	cmd.Flags().BoolVarP(&cli.config.LocalFlags.PathTypes, "types", "", false, "print leaf type")
	cmd.Flags().BoolVarP(&cli.config.LocalFlags.PathSearch, "search", "", false, "search through path list")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		cli.config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
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
		if e.IsCase() || e.IsChoice() {
			continue
		}
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
	if cli.config.LocalFlags.PathPathType == "gnmi" {
		gnmiPath, err := xpath.ToGNMIPath(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "path: %s could not be changed to gnmi: %v\n", path, err)
		}
		path = gnmiPath.String()
	}
	if cli.config.LocalFlags.PathTypes {
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
		if cli.config.LocalFlags.PathWithPrefix {
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
			unionlist = append(unionlist, t.Type[i].Name)
		}
		rstr += fmt.Sprintf(" %v", unionlist)
	default:
	}
	rstr += "\n"

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
		rstr += "- required\n"
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

func walkDir(path, ext string) ([]string, error) {
	fs := make([]string, 0)
	err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fi, err := os.Stat(path)
			if err != nil {
				return err
			}
			switch mode := fi.Mode(); {
			case mode.IsRegular():
				if filepath.Ext(path) == ext {
					fs = append(fs, path)
				}
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func findYangFiles(files []string) ([]string, error) {
	yfiles := make([]string, 0, len(files))
	for _, file := range files {
		fi, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		switch mode := fi.Mode(); {
		case mode.IsDir():
			fls, err := walkDir(file, ".yang")
			if err != nil {
				return nil, err
			}
			yfiles = append(yfiles, fls...)
		case mode.IsRegular():
			if filepath.Ext(file) == ".yang" {
				yfiles = append(yfiles, file)
			}
		}
	}
	return yfiles, nil
}
