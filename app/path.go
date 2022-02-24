package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/gnxi/utils/xpath"
	"github.com/karimra/gnmic/utils"
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
	json       bool
}

type generatedPath struct {
	Path           string `json:"path,omitempty"`
	PathWithPrefix string `json:"path-with-prefix,omitempty"`
	Type           string `json:"type,omitempty"`
	Description    string `json:"description,omitempty"`
	Default        string `json:"default,omitempty"`
	IsState        bool   `json:"is-state,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
}

func (a *App) PathCmdRun(d, f, e []string, pgo pathGenOpts) error {
	err := a.generateYangSchema(d, f, e)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan *generatedPath)
	gpaths := make([]*generatedPath, 0)

	go func(ctx context.Context, out chan *generatedPath) {
		for {
			select {
			case m, ok := <-out:
				if !ok {
					return
				}
				gpaths = append(gpaths, m)
			case <-ctx.Done():
				return
			}
		}
	}(ctx, out)

	collected := make([]*yang.Entry, 0, 256)
	for _, entry := range a.SchemaTree.Dir {
		collected = append(collected, collectSchemaNodes(entry, true)...)
	}
	for _, entry := range collected {
		if !pgo.stateOnly && !pgo.configOnly || pgo.stateOnly && pgo.configOnly {
			out <- a.generatePath(entry, pgo.pathType)
			continue
		}
		state := isState(entry)
		if state && pgo.stateOnly {
			out <- a.generatePath(entry, pgo.pathType)
			continue
		}
		if !state && pgo.configOnly {
			out <- a.generatePath(entry, pgo.pathType)
			continue
		}
	}
	close(out)
	sort.Slice(gpaths, func(i, j int) bool {
		return gpaths[i].Path < gpaths[j].Path
	})
	for _, gp := range gpaths {
		gp.PathWithPrefix = collapsePrefixes(gp.PathWithPrefix)
	}
	if pgo.json {
		b, err := json.MarshalIndent(gpaths, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(b))
		return nil
	}

	if len(gpaths) == 0 {
		return errors.New("no results found")
	}

	// regular print
	if !pgo.search {
		sb := new(strings.Builder)
		for _, gp := range gpaths {
			sb.Reset()
			sb.WriteString(gp.Path)
			if pgo.withTypes {
				sb.WriteString("\t(type=")
				sb.WriteString(gp.Type)
				sb.WriteString(")")
			}
			if pgo.withDescr {
				sb.WriteString("\n")
				sb.WriteString(indent("\t", gp.Description))
			}
			fmt.Fprintln(os.Stdout, sb.String())
		}
		return nil
	}
	// search
	paths := make([]string, 0, len(gpaths))
	for _, gp := range gpaths {
		paths = append(paths, gp.Path)
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

func (a *App) generatePath(entry *yang.Entry, pType string) *generatedPath {
	gp := new(generatedPath)
	for e := entry; e != nil && e.Parent != nil; e = e.Parent {
		if e.IsCase() || e.IsChoice() {
			continue
		}
		elementName := e.Name
		prefixedElementName := e.Name
		if e.Prefix != nil {
			prefixedElementName = fmt.Sprintf("%s:%s", e.Prefix.Name, prefixedElementName)
		}
		if e.Key != "" {
			for _, k := range strings.Fields(e.Key) {
				elementName = fmt.Sprintf("%s[%s=*]", elementName, k)
				prefixedElementName = fmt.Sprintf("%s[%s=*]", prefixedElementName, k)
			}
		}
		gp.Path = fmt.Sprintf("/%s%s", elementName, gp.Path)
		if e.Prefix != nil {
			gp.PathWithPrefix = fmt.Sprintf("/%s%s", prefixedElementName, gp.PathWithPrefix)
		}
	}

	gp.Description = entry.Description
	gp.Type = entry.Type.Name

	if entry.IsLeafList() {
		gp.Default = strings.Join(entry.DefaultValues(), ", ")
	} else {
		gp.Default, _ = entry.SingleDefaultValue()
	}

	gp.IsState = isState(entry)
	gp.Namespace = entry.Namespace().NName()
	if pType == "gnmi" {
		gnmiPath, err := xpath.ToGNMIPath(gp.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "path: %s could not be changed to gnmi format: %v\n", gp.Path, err)
		}
		gp.Path = gnmiPath.String()
	}
	return gp
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

func isState(e *yang.Entry) bool {
	if e.Config == yang.TSFalse {
		return true
	}
	if e.Parent != nil {
		return isState(e.Parent)
	}
	return false
}

// collapsePrefixes removes prefixes from path element names and keys
func collapsePrefixes(p string) string {
	gp, err := utils.ParsePath(p)
	if err != nil {
		return p
	}
	parentPrefix := ""
	for _, pe := range gp.Elem {
		currentPrefix, name := getPrefixElem(pe.Name)
		if parentPrefix == "" || parentPrefix != currentPrefix {
			// first elem or updating parent prefix
			parentPrefix = currentPrefix
		} else if currentPrefix == parentPrefix {
			pe.Name = name
		}
	}
	return fmt.Sprintf("/%s", utils.GnmiPathToXPath(gp, false))
}

// takes a path element name or a key name
// and returns the prefix and name
func getPrefixElem(pe string) (string, string) {
	if pe == "" {
		return "", ""
	}
	pes := strings.SplitN(pe, ":", 2)
	if len(pes) > 1 {
		return pes[0], pes[1]
	}
	return "", pes[0]
}
