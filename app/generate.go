package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/config"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

func (a *App) GeneratePreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	a.Config.LocalFlags.GenerateDir = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GenerateDir)
	a.Config.LocalFlags.GenerateFile = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GenerateFile)
	a.Config.LocalFlags.GenerateExclude = config.SanitizeArrayFlagValue(a.Config.LocalFlags.GenerateExclude)

	var err error
	a.Config.LocalFlags.GenerateDir, err = resolveGlobs(a.Config.LocalFlags.GenerateDir)
	if err != nil {
		return err
	}
	a.Config.LocalFlags.GenerateFile, err = resolveGlobs(a.Config.LocalFlags.GenerateFile)
	if err != nil {
		return err
	}
	for _, dirpath := range a.Config.LocalFlags.GenerateDir {
		expanded, err := yang.PathsWithModules(dirpath)
		if err != nil {
			return err
		}
		if a.Config.Debug {
			for _, fdir := range expanded {
				a.Logger.Printf("adding %s to YANG paths", fdir)
			}
		}
		yang.AddPath(expanded...)
	}
	yfiles, err := findYangFiles(a.Config.LocalFlags.GenerateFile)
	if err != nil {
		return err
	}
	a.Config.LocalFlags.GenerateFile = make([]string, 0, len(yfiles))
	a.Config.LocalFlags.GenerateFile = append(a.Config.LocalFlags.PathFile, yfiles...)
	if a.Config.Debug {
		for _, file := range a.Config.LocalFlags.GenerateFile {
			a.Logger.Printf("loading %s file", file)
		}
	}
	return nil
}

func (a *App) GenerateSetRequestRunE(cmd *cobra.Command, args []string) error {
	defer a.InitGenerateSetRequestFlags(cmd)
	var output = os.Stdout
	if a.Config.GenerateOutput != "" {
		f, err := os.OpenFile(a.Config.GenerateOutput, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
		output = f
	}
	err := a.GenerateYangSchema(a.Config.GenerateDir, a.Config.GenerateFile, a.Config.GenerateExclude)
	if err != nil {
		return err
	}
	m := make(map[string]interface{})
	for _, e := range a.SchemaTree.Dir {
		e.FixChoice()
		nm := toMap(e)
		if nm == nil {
			continue
		}

		switch nm := nm.(type) {
		case map[string]interface{}:
			for k, v := range nm {
				m[k] = v
			}
		case []interface{}:
			m[e.Name] = nm
		case string:
			m[e.Name] = nm
		}
	}

	setReqFile, err := a.createSetRequestFile(m)
	if err != nil {
		return err
	}

	err = output.Truncate(0)
	if err != nil {
		return err
	}

	return yaml.NewEncoder(output).Encode(setReqFile)
}

func (a *App) InitGenerateFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.PersistentFlags().StringArrayVarP(&a.Config.LocalFlags.GenerateFile, "file", "", []string{}, "yang file(s)")
	cmd.PersistentFlags().StringArrayVarP(&a.Config.LocalFlags.GenerateDir, "dir", "", []string{}, "yang dir(s)")
	cmd.PersistentFlags().StringArrayVarP(&a.Config.LocalFlags.GenerateExclude, "exclude", "", []string{}, "regexes defining modules to be excluded")
	cmd.PersistentFlags().StringVarP(&a.Config.LocalFlags.GenerateOutput, "output", "", "", "output file, defaults to stdout")
	cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func (a *App) InitGenerateSetRequestFlags(cmd *cobra.Command) {
	cmd.ResetFlags()
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.GenerateSetRequestReplacePath, "replace", "", []string{}, "replace path")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.GenerateSetRequestUpdatePath, "update", "", []string{}, "update path")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func (a *App) GenerateYangSchema(dirs, files, excludes []string) error {
	if len(files) == 0 {
		return nil
	}

	ms := yang.NewModules()
	for _, name := range files {
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

	a.SchemaTree = buildRootEntry()
	excludeRegexes := make([]*regexp.Regexp, 0, len(excludes))
	for _, e := range excludes {
		r, err := regexp.Compile(e)
		if err != nil {
			return err
		}
		excludeRegexes = append(excludeRegexes, r)
	}
	for _, entry := range entries {
		skip := false
		for _, r := range excludeRegexes {
			if r.MatchString(entry.Name) {
				a.Logger.Printf("skipping %s", entry.Name)
				skip = true
				break
			}
		}
		if !skip {
			updateAnnotation(entry)
			a.SchemaTree.Dir[entry.Name] = entry
		}
	}
	return nil
}

func (a *App) createSetRequestFile(m map[string]interface{}) (*config.SetRequestFile, error) {
	setReqFile := &config.SetRequestFile{
		Replaces: make([]*config.UpdateItem, 0, len(a.Config.GenerateSetRequestReplacePath)),
		Updates:  make([]*config.UpdateItem, 0, len(a.Config.GenerateSetRequestUpdatePath)),
	}
	var enc string
	if strings.ToUpper(a.Config.Encoding) != "JSON" {
		enc = strings.ToUpper(a.Config.Encoding)
	}
	if len(a.Config.GenerateSetRequestReplacePath)+len(a.Config.GenerateSetRequestUpdatePath) == 0 {
		sortedKeys := make([]string, 0, len(m))
		for k := range m {
			sortedKeys = append(sortedKeys, k)
		}

		sort.Strings(sortedKeys)
		for _, n := range sortedKeys {
			setReqFile.Replaces = append(setReqFile.Replaces,
				&config.UpdateItem{
					Path:     fmt.Sprintf("/%s", n),
					Encoding: enc,
					Value:    m[n],
				})
		}
		return setReqFile, nil
	}
	for _, p := range a.Config.GenerateSetRequestReplacePath {
		uItem, err := pathToUpdateItem(p, m)
		if err != nil {
			return nil, err
		}
		uItem.Encoding = enc
		setReqFile.Replaces = append(setReqFile.Replaces, uItem)
	}
	for _, p := range a.Config.GenerateSetRequestUpdatePath {
		uItem, err := pathToUpdateItem(p, m)
		if err != nil {
			return nil, err
		}
		uItem.Encoding = enc
		setReqFile.Updates = append(setReqFile.Updates, uItem)
	}
	return setReqFile, nil
}

func buildRootEntry() *yang.Entry {
	return &yang.Entry{
		Name: "root",
		Kind: yang.DirectoryEntry,
		Dir:  make(map[string]*yang.Entry),
		Annotation: map[string]interface{}{
			"schemapath": "/",
			"root":       true,
		},
	}
}

// updateAnnotation updates the schema info before encoding.
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

func toMap(e *yang.Entry) interface{} {
	if e == nil {
		return nil
	}
	if e.Config == yang.TSFalse {
		return nil
	}
	m := make(map[string]interface{})
	switch {
	case e.Dir == nil && e.ListAttr != nil: // leaf-list
		fallthrough
	case e.Dir == nil: // leaf
		if e.Config == yang.TSFalse {
			return nil
		}
		return e.Default
	case e.ListAttr != nil: // list
		for n, child := range e.Dir {
			gChild := toMap(child)
			switch gChild := gChild.(type) {
			case map[string]interface{}:
				for k, v := range gChild {
					m[k] = v
				}
			case []interface{}:
				m[n] = gChild
			case string:
				m[n] = gChild
			}
		}
		return []interface{}{m}
	default: // container
		nm := make(map[string]interface{})
		for n, child := range e.Dir {
			if child.IsCase() || child.IsChoice() {
				for _, gchild := range child.Dir {
					nnm := toMap(gchild)
					switch nnm := nnm.(type) {
					case map[string]interface{}:
						if child.IsChoice() {
							for k, v := range nnm {
								nm[k] = v
							}
						}
					case nil:
					default:
						nm[n] = nnm
					}
				}
				continue
			}
			nnm := toMap(child)
			if nnm == nil {
				continue
			}
			nm[n] = nnm
		}
		if e.Parent != nil && e.Parent.IsList() && !(e.IsCase() || e.IsChoice()) {
			m[e.Name] = nm
			return m
		}
		for k, v := range nm {
			m[k] = v
		}
		return m
	}
}

func pathToUpdateItem(p string, m map[string]interface{}) (*config.UpdateItem, error) {
	// strip path from keys if any
	gp, err := collector.ParsePath(p)
	if err != nil {
		return nil, fmt.Errorf("failed to parse xpath %q: %v", p, err)
	}
	pItems := make([]string, 0, len(gp.Elem))
	for _, e := range gp.Elem {
		if e.Name != "" {
			pItems = append(pItems, e.Name)
		}
	}
	// get value body recursively from map
	var rVal interface{}
	rVal = m
	for _, item := range pItems {
		switch rValm := rVal.(type) {
		case map[string]interface{}:
			if r, ok := rValm[item]; ok {
				rVal = r
			} else {
				return nil, fmt.Errorf("unknown path item %q in path %q", item, p)
			}
		case []interface{}:
			if len(rValm) != 1 {
				return nil, fmt.Errorf("got list with more than 1 item ?")
			}
			switch rValmn := rValm[0].(type) {
			case map[string]interface{}:
				if r, ok := rValmn[item]; ok {
					rVal = r
				} else {
					return nil, fmt.Errorf("unknown path item %q in path %q", item, p)
				}
			}
		default:
			return nil, fmt.Errorf("unexpected sub map format @%q: %T", item, rVal)
		}
	}
	return &config.UpdateItem{
		Path:  p,
		Value: rVal,
	}, nil
}

//////

func resolveGlobs(globs []string) ([]string, error) {
	results := make([]string, 0, len(globs))
	for _, pattern := range globs {
		for _, p := range strings.Split(pattern, ",") {
			if strings.ContainsAny(p, `*?[`) {
				// is a glob pattern
				matches, err := filepath.Glob(p)
				if err != nil {
					return nil, err
				}
				results = append(results, matches...)
			} else {
				// is not a glob pattern ( file or dir )
				results = append(results, p)
			}
		}
	}
	return config.ExpandOSPaths(results)
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
