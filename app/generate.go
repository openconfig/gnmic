package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/huandu/xstrings"
	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

// options for formatting keys when generating yaml/json payloads
type keyOpts struct {
	camelCase bool
	snakeCase bool
}

func (ko *keyOpts) format(s string) string {
	if ko.camelCase {
		return xstrings.ToCamelCase(s)
	}
	if ko.snakeCase {
		return xstrings.ToSnakeCase(s)
	}
	return s
}

func (a *App) GenerateRunE(cmd *cobra.Command, args []string) error {
	defer a.InitGenerateFlags(cmd)
	var output = os.Stdout
	if a.Config.GenerateOutput != "" {
		f, err := os.OpenFile(a.Config.GenerateOutput, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
		output = f
	}
	err := a.generateYangSchema(a.Config.GlobalFlags.Dir, a.Config.GlobalFlags.File, a.Config.GlobalFlags.Exclude)
	if err != nil {
		return err
	}
	m := make(map[string]interface{})
	kOpts := &keyOpts{
		camelCase: a.Config.LocalFlags.GenerateCamelCase,
		snakeCase: a.Config.LocalFlags.GenerateSnakeCase,
	}
	for _, e := range a.SchemaTree.Dir {
		e.FixChoice()
		nm := toMap(e, a.Config.GenerateConfigOnly, kOpts)
		if nm == nil {
			continue
		}

		switch nm := nm.(type) {
		case map[string]interface{}:
			for k, v := range nm {
				m[kOpts.format(k)] = v
			}
		case []interface{}, string:
			m[kOpts.format(e.Name)] = nm
		}
	}

	v, err := getSubMapByPath(a.Config.GeneratePath, m, kOpts)
	if err != nil {
		return err
	}
	if output != os.Stdout {
		err = output.Truncate(0)
		if err != nil {
			return err
		}
	}
	if a.Config.GenerateJSON {
		enc := json.NewEncoder(output)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	return yaml.NewEncoder(output).Encode(v)
}

func (a *App) GeneratePreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	if a.Config.LocalFlags.GenerateCamelCase && a.Config.LocalFlags.GenerateSnakeCase {
		return errors.New("flags --camel-case and --snake-case are mutually exclusive")
	}
	return a.yangFilesPreProcessing()
}

func (a *App) yangFilesPreProcessing() error {
	a.Config.GlobalFlags.Dir = config.SanitizeArrayFlagValue(a.Config.GlobalFlags.Dir)
	a.Config.GlobalFlags.File = config.SanitizeArrayFlagValue(a.Config.GlobalFlags.File)
	a.Config.GlobalFlags.Exclude = config.SanitizeArrayFlagValue(a.Config.GlobalFlags.Exclude)

	var err error
	a.Config.GlobalFlags.Dir, err = resolveGlobs(a.Config.GlobalFlags.Dir)
	if err != nil {
		return err
	}
	a.Config.GlobalFlags.File, err = resolveGlobs(a.Config.GlobalFlags.File)
	if err != nil {
		return err
	}
	a.modules = yang.NewModules()
	for _, dirpath := range a.Config.GlobalFlags.Dir {
		expanded, err := yang.PathsWithModules(dirpath)
		if err != nil {
			return err
		}
		if a.Config.Debug {
			for _, fdir := range expanded {
				a.Logger.Printf("adding %s to YANG paths", fdir)
			}
		}
		a.modules.AddPath(expanded...)
	}
	yfiles, err := findYangFiles(a.Config.GlobalFlags.File)
	if err != nil {
		return err
	}
	a.Config.GlobalFlags.File = make([]string, 0, len(yfiles))
	a.Config.GlobalFlags.File = append(a.Config.GlobalFlags.File, yfiles...)
	if a.Config.Debug {
		for _, file := range a.Config.GlobalFlags.File {
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
	err := a.generateYangSchema(a.Config.GlobalFlags.Dir, a.Config.GlobalFlags.File, a.Config.GlobalFlags.Exclude)
	if err != nil {
		return err
	}
	m := make(map[string]interface{})
	for _, e := range a.SchemaTree.Dir {
		e.FixChoice()
		nm := toMap(e, true, new(keyOpts))
		if nm == nil {
			continue
		}

		switch nm := nm.(type) {
		case map[string]interface{}:
			for k, v := range nm {
				m[k] = v
			}
		case nil:
		default:
			m[e.Name] = nm
		}
	}

	setReqFile, err := a.createSetRequestFile(m)
	if err != nil {
		return err
	}
	if output != os.Stdout {
		err = output.Truncate(0)
		if err != nil {
			return err
		}
	}
	if a.Config.GenerateJSON {
		enc := json.NewEncoder(output)
		enc.SetIndent("", "  ")
		return enc.Encode(setReqFile)
	}
	return yaml.NewEncoder(output).Encode(setReqFile)
}

func (a *App) InitGenerateFlags(cmd *cobra.Command) {
	cmd.ResetFlags()
	// persistent flags
	cmd.PersistentFlags().StringVarP(&a.Config.LocalFlags.GenerateOutput, "output", "o", "", "output file, defaults to stdout")
	cmd.PersistentFlags().BoolVarP(&a.Config.LocalFlags.GenerateJSON, "json", "j", false, "generate output as JSON format instead of YAML")
	// local flags
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GenerateConfigOnly, "config-only", "", false, "generate output from YANG config nodes only")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.GeneratePath, "path", "", "", "generate marshaled YANG body under specified path")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GenerateCamelCase, "camel-case", "", false, "convert keys to camelCase")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.GenerateSnakeCase, "snake-case", "", false, "convert keys to snake_case")

	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
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

func (a *App) generateYangSchema(dirs, files, excludes []string) error {
	if len(files) == 0 {
		return nil
	}

	for _, name := range files {
		if err := a.modules.Read(name); err != nil {
			return err
		}
	}
	if errors := a.modules.Process(); len(errors) > 0 {
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "yang processing error: %v\n", e)
		}
		return fmt.Errorf("yang processing failed with %d errors", len(errors))
	}
	// Keep track of the top level modules we read in.
	// Those are the only modules we want to print below.
	mods := map[string]*yang.Module{}
	var names []string

	for _, m := range a.modules.Modules {
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
		uItem, err := pathToUpdateItem(p, m, new(keyOpts))
		if err != nil {
			return nil, err
		}
		uItem.Encoding = enc
		setReqFile.Replaces = append(setReqFile.Replaces, uItem)
	}
	for _, p := range a.Config.GenerateSetRequestUpdatePath {
		uItem, err := pathToUpdateItem(p, m, new(keyOpts))
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

func toMap(e *yang.Entry, configOnly bool, kopts *keyOpts) interface{} {
	if e == nil {
		return nil
	}
	if e.Config == yang.TSFalse && configOnly {
		return nil
	}
	m := make(map[string]interface{})
	switch {
	case e.Dir == nil && e.ListAttr != nil: // leaf-list
		fallthrough
	case e.Dir == nil: // leaf
		if e.Config == yang.TSFalse && configOnly {
			return nil
		}
		return e.Default
	case e.ListAttr != nil: // list
		for n, child := range e.Dir {
			gChild := toMap(child, configOnly, kopts)
			switch gChild := gChild.(type) {
			case map[string]interface{}:
				for k, v := range gChild {
					m[kopts.format(k)] = v
				}
			case []interface{}, string:
				m[kopts.format(n)] = gChild
			}
		}
		return []interface{}{m}
	default: // container
		nm := make(map[string]interface{})
		for n, child := range e.Dir {
			if child.IsCase() || child.IsChoice() {
				for _, gchild := range child.Dir {
					nnm := toMap(gchild, configOnly, kopts)
					switch nnm := nnm.(type) {
					case map[string]interface{}:
						if child.IsChoice() {
							for k, v := range nnm {
								nm[kopts.format(k)] = v
							}
						}
					case nil:
					default:
						nm[kopts.format(n)] = nnm
					}
				}
				continue
			}
			nnm := toMap(child, configOnly, kopts)
			if nnm == nil {
				continue
			}
			nm[kopts.format(n)] = nnm
		}
		if e.Parent != nil && e.Parent.IsList() && !(e.IsCase() || e.IsChoice()) {
			m[kopts.format(e.Name)] = nm
			return m
		}
		for k, v := range nm {
			m[kopts.format(k)] = v
		}
		return m
	}
}

func pathToUpdateItem(p string, m map[string]interface{}, kopts *keyOpts) (*config.UpdateItem, error) {
	v, err := getSubMapByPath(p, m, kopts)
	return &config.UpdateItem{
		Path:  p,
		Value: v,
	}, err
}

func getSubMapByPath(p string, m map[string]interface{}, kopts *keyOpts) (interface{}, error) {
	if p == "" || p == "/" {
		return m, nil
	}
	// strip path from keys if any
	gp, err := utils.ParsePath(p)
	if err != nil {
		return nil, fmt.Errorf("failed to parse xpath %q: %v", p, err)
	}
	pItems := make([]string, 0, len(gp.Elem))
	for _, e := range gp.Elem {
		if e.Name != "" {
			pItems = append(pItems, kopts.format(e.Name))
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
	return rVal, nil
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
