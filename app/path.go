package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/gnxi/utils/xpath"
	"github.com/manifoldco/promptui"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type pathGenOpts struct {
	search     bool
	withDescr  bool
	withTypes  bool
	withPrefix bool
	pathType   string
	stateOnly  bool
	configOnly bool
}

func (a *App) PathCmdRun(d, f, e []string, pgo pathGenOpts) error {
	err := a.GenerateYangSchema(d, f, e)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan string)
	paths := make([]string, 0)
	if pgo.search {
		go gather(ctx, out, &paths)
	} else {
		go printer(ctx, out)
	}
	collected := make([]*yang.Entry, 0, 256)
	for _, entry := range a.SchemaTree.Dir {
		collected = append(collected, collectSchemaNodes(entry, true)...)
	}
	for _, entry := range collected {
		if !pgo.stateOnly && !pgo.configOnly || pgo.stateOnly && pgo.configOnly {
			out <- a.generatePath(entry, pgo.withDescr, pgo.withPrefix, pgo.withTypes, pgo.pathType)
			continue
		}
		state := isState(entry)
		if state && pgo.stateOnly {
			out <- a.generatePath(entry, pgo.withDescr, pgo.withPrefix, pgo.withTypes, pgo.pathType)
			continue
		}
		if !state && pgo.configOnly {
			out <- a.generatePath(entry, pgo.withDescr, pgo.withPrefix, pgo.withTypes, pgo.pathType)
			continue
		}
	}
	close(out)
	if pgo.search {
		if len(paths) == 0 {
			return errors.New("no results found")
		}
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
		fmt.Println(a.generateTypeInfo(collected[index]))
	}
	return nil
}

func (a *App) PathPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	if a.Config.PathSearch && a.Config.PathWithDescr {
		return errors.New("flags --search and --descr cannot be used together")
	}
	if a.Config.LocalFlags.PathPathType != "xpath" && a.Config.LocalFlags.PathPathType != "gnmi" {
		return errors.New("path-type must be one of 'xpath' or 'gnmi'")
	}
	return a.yangFilesPreProcessing()
}

func (a *App) PathRunE(cmd *cobra.Command, args []string) error {
	return a.PathCmdRun(
		a.Config.GlobalFlags.Dir,
		a.Config.GlobalFlags.File,
		a.Config.GlobalFlags.Exclude,
		pathGenOpts{
			search:     a.Config.LocalFlags.PathSearch,
			withDescr:  a.Config.LocalFlags.PathWithDescr,
			withTypes:  a.Config.LocalFlags.PathWithTypes,
			withPrefix: a.Config.LocalFlags.PathWithPrefix,
			pathType:   a.Config.LocalFlags.PathPathType,
			stateOnly:  a.Config.LocalFlags.PathState,
			configOnly: a.Config.LocalFlags.PathConfig,
		},
	)
}

func (a *App) InitPathFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&a.Config.LocalFlags.PathPathType, "path-type", "", "xpath", "path type xpath or gnmi")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.PathWithDescr, "descr", "", false, "print leaf description")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.PathWithPrefix, "with-prefix", "", false, "include module/submodule prefix in path elements")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.PathWithTypes, "types", "", false, "print leaf type")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.PathSearch, "search", "", false, "search through path list")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.PathState, "state-only", "", false, "generate paths only for YANG leafs representing state data")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.PathConfig, "config-only", "", false, "generate paths only for YANG leafs representing config data")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
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

func (a *App) generatePath(entry *yang.Entry, withDescr, prefixTagging, withTypes bool, pType string) string {
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
			for _, k := range strings.Fields(e.Key) {
				if prefixTagging && e.Prefix != nil {
					k = e.Prefix.Name + ":" + k
				}
				elementName = fmt.Sprintf("%s[%s=*]", elementName, k)
			}
		}
		path = fmt.Sprintf("/%s%s", elementName, path)
	}
	if pType == "gnmi" {
		gnmiPath, err := xpath.ToGNMIPath(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "path: %s could not be changed to gnmi: %v\n", path, err)
		}
		path = gnmiPath.String()
	}
	if withDescr {
		path = fmt.Sprintf("%s\n%s", path, indent("\t", entry.Description))
	}
	if withTypes {
		path = fmt.Sprintf("%s (type=%s)", path, entry.Type.Name)
	}
	return path
}

func (a *App) generateTypeInfo(e *yang.Entry) string {
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
		if a.Config.LocalFlags.PathWithPrefix {
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

func printer(ctx context.Context, c chan string) {
	for {
		select {
		case m, ok := <-c:
			if !ok {
				return
			}
			fmt.Println(m)
		case <-ctx.Done():
			return
		}
	}
}

func gather(ctx context.Context, c chan string, ls *[]string) {
	for {
		select {
		case m, ok := <-c:
			if !ok {
				return
			}
			*ls = append(*ls, m)
		case <-ctx.Done():
			return
		}
	}
}

func isState(e *yang.Entry) bool {
	if e.Config == yang.TSFalse {
		return true
	}
	if e.Parent != nil {
		return isState(e.Parent)
	}
	return false
}
