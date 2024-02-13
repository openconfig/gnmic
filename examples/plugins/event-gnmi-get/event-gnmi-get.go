package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/openconfig/gnmic/pkg/api"
	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/formatters/event_plugin"
	gtemplate "github.com/openconfig/gnmic/pkg/gtemplate"
	gpath "github.com/openconfig/gnmic/pkg/api/path"
)

const (
	processorType = "event-gnmi-get"
	loggingPrefix = "[" + processorType + "] "
	defaultTarget = `{{ index .Tags "source" }}`
)

type gNMIGetProcessor struct {
	Debug        bool          `mapstructure:"debug,omitempty" yaml:"debug,omitempty" json:"debug,omitempty"`
	ReadPeriod   time.Duration `mapstructure:"read-period,omitempty" yaml:"read-period,omitempty" json:"read-period,omitempty"`
	Target       string        `mapstructure:"target,omitempty" yaml:"target,omitempty" json:"target,omitempty"`
	PrefixTarget string        `mapstructure:"prefix-target,omitempty" yaml:"prefix-target,omitempty" json:"prefix-target,omitempty"`
	Paths        []*pathToTag  `mapstructure:"paths,omitempty" yaml:"paths,omitempty" json:"paths,omitempty"`
	Type         string        `mapstructure:"data-type,omitempty" yaml:"type,omitempty" json:"type,omitempty"`
	Encoding     string        `mapstructure:"encoding,omitempty" yaml:"encoding,omitempty" json:"encoding,omitempty"`
	SkipVerify   bool          `mapstructure:"skip-verify,omitempty" yaml:"skip-verify,omitempty" json:"skip-verify,omitempty"`

	m               *sync.RWMutex
	prefixTargetTpl *template.Template
	targetTpl       *template.Template
	// values read indexed by targetName
	vals map[string]*readValues

	targetsConfigs        map[string]*types.TargetConfig
	actionsDefinitions    map[string]map[string]interface{}
	processorsDefinitions map[string]map[string]any
	logger                hclog.Logger
}

type pathToTag struct {
	Path    string `mapstructure:"path,omitempty" yaml:"path,omitempty" json:"path,omitempty"`
	TagName string `mapstructure:"tag-name,omitempty" yaml:"tag-name,omitempty" json:"tag-name,omitempty"`

	pathTpl *template.Template
}

type readValues struct {
	vals     map[string]string
	lastRead time.Time
}

func (p *gNMIGetProcessor) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, o := range opts {
		o(p)
	}
	p.setupLogger()
	p.logger.Info("initializing", "processor", processorType, "cfg", cfg)
	if p.Target == "" {
		p.Target = defaultTarget
	}
	if p.ReadPeriod <= 0 {
		p.ReadPeriod = time.Minute
	}
	if p.Type == "" {
		p.Type = "all"
	}
	// init PrefixTarget if any
	if p.PrefixTarget != "" {
		p.prefixTargetTpl, err = gtemplate.CreateTemplate(fmt.Sprintf("%s-prefix-target", processorType), p.PrefixTarget)
		if err != nil {
			return err
		}
	}
	// init target template
	p.targetTpl, err = gtemplate.CreateTemplate(fmt.Sprintf("%s-target", processorType), p.Target)
	if err != nil {
		return err
	}
	// init paths templates
	for i, pd := range p.Paths {
		pd.pathTpl, err = gtemplate.CreateTemplate(fmt.Sprintf("path-%d", i), pd.Path)
		if err != nil {
			return err
		}
	}
	p.logger.Info("initialized", "processor", processorType)
	return nil
}

func (p *gNMIGetProcessor) Apply(event ...*formatters.EventMsg) []*formatters.EventMsg {
	p.m.Lock()
	defer p.m.Unlock()

	for _, e := range event {
		targetName, err := p.readPaths(e)
		if err != nil {
			p.logger.Error("failed to read paths", "error", err)
		}
		if _, ok := p.vals[targetName]; !ok {
			p.logger.Error("unknown target", "target", targetName)
			continue
		}
		if e.Tags == nil {
			e.Tags = make(map[string]string)
		}
		for k, v := range p.vals[targetName].vals {
			e.Tags[k] = v
		}
	}
	return event
}

func (p *gNMIGetProcessor) WithActions(act map[string]map[string]interface{}) {
	p.actionsDefinitions = act
}

func (p *gNMIGetProcessor) WithTargets(tcs map[string]*types.TargetConfig) {
	p.targetsConfigs = tcs
}

func (p *gNMIGetProcessor) WithProcessors(procs map[string]map[string]any) {
	p.processorsDefinitions = procs
}

func (p *gNMIGetProcessor) WithLogger(l *log.Logger) {
}

func (p *gNMIGetProcessor) setupLogger() {
	p.logger = hclog.New(&hclog.LoggerOptions{
		Output:     os.Stderr,
		TimeFormat: "2006/01/02 15:04:05.999999",
	})

	if p.Debug {
		p.logger.SetLevel(hclog.Debug)
	}
}

func (p *gNMIGetProcessor) readPaths(e *formatters.EventMsg) (string, error) {
	now := time.Now()

	var err error
	b := new(bytes.Buffer)
	switch {
	case p.prefixTargetTpl != nil:
		err = p.prefixTargetTpl.Execute(b, e)
	case p.targetTpl != nil:
		err = p.targetTpl.Execute(b, e)
	}
	if err != nil {
		return "", err
	}
	targetName := b.String()
	_, ok := p.vals[targetName]
	if !ok {
		p.vals[targetName] = &readValues{
			vals: map[string]string{},
		}
	}
	if p.vals[targetName].lastRead.After(now.Add(-p.ReadPeriod)) {
		return targetName, nil
	}
	//
	vals, err := p.gnmiGet(targetName, e)
	if err != nil {
		return "", err
	}
	p.logger.Debug("vals from get", "vals", vals)
	p.vals[targetName].vals = vals
	p.vals[targetName].lastRead = time.Now()
	return targetName, nil
}

func (p *gNMIGetProcessor) gnmiGet(targetName string, e *formatters.EventMsg) (map[string]string, error) {
	tc, err := p.selectTarget(targetName)
	if err != nil {
		return nil, err
	}

	t := target.NewTarget(tc)
	req, keyPathMapping, err := p.createGetRequest(e)
	if err != nil {
		return nil, err
	}
	p.logger.Debug("keyPathMapping", "mapping", keyPathMapping)

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	err = t.CreateGNMIClient(ctx)
	if err != nil {
		return nil, err
	}
	defer t.Close()

	resp, err := t.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("target %q GetRequest failed: %v", t.Config.Name, err)
	}

	return p.extractTags(resp, keyPathMapping), nil
}

func (p *gNMIGetProcessor) selectTarget(tName string) (*types.TargetConfig, error) {
	if tName == "" {
		return nil, fmt.Errorf("target name is empty")
	}
	if p.prefixTargetTpl != nil {
		tc := &types.TargetConfig{
			Name:       p.Target,
			Address:    p.Target,
			SkipVerify: pointer.ToBool(p.SkipVerify),
			Timeout:    10 * time.Second,
		}
		if !p.SkipVerify {
			tc.Insecure = pointer.ToBool(true)
		}
		return tc, nil
	}
	if tc, ok := p.targetsConfigs[tName]; ok {
		return tc, nil
	}
	return nil, fmt.Errorf("unknown target %s", tName)
}

func (p *gNMIGetProcessor) createGetRequest(e *formatters.EventMsg) (*gnmi.GetRequest, map[string]string, error) {
	gnmiOpts := make([]api.GNMIOption, 0, 3)
	gnmiOpts = append(gnmiOpts, api.Encoding(p.Encoding))
	gnmiOpts = append(gnmiOpts, api.DataType(p.Type))

	var err error
	b := new(bytes.Buffer)
	if p.prefixTargetTpl != nil {
		err = p.prefixTargetTpl.Execute(b, e)
		if err != nil {
			return nil, nil, fmt.Errorf("prefix-target parse error: %v", err)
		}
		ps := b.String()
		gnmiOpts = append(gnmiOpts, api.Target(ps))
	}

	pathToKey := map[string]string{}
	for _, ptt := range p.Paths {
		b.Reset()
		err = ptt.pathTpl.Execute(b, e)
		if err != nil {
			return nil, nil, fmt.Errorf("path parse error: %v", err)
		}
		ps := b.String()
		gnmiOpts = append(gnmiOpts, api.Path(ps))
		pathToKey[ps] = ptt.TagName
	}
	req, err := api.NewGetRequest(gnmiOpts...)
	if err != nil {
		return nil, nil, err
	}
	return req, pathToKey, nil
}

func (p *gNMIGetProcessor) extractTags(rsp *gnmi.GetResponse, mapping map[string]string) map[string]string {
	rs := map[string]string{}
	for _, n := range rsp.GetNotification() {
		for _, upd := range n.GetUpdate() {

			xp := gpath.GnmiPathToXPath(upd.GetPath(), false)
			p.logger.Debug("path", "xp", xp, "v", upd.GetVal())
			if k, ok := mapping[xp]; ok {
				rs[k] = extractValue(upd.GetVal())
			}
		}
	}
	return rs
}

func extractValue(tv *gnmi.TypedValue) string {
	switch tv.Value.(type) {
	case *gnmi.TypedValue_AsciiVal:
		return tv.GetAsciiVal()
	case *gnmi.TypedValue_BoolVal:
		return fmt.Sprintf("%t", tv.GetBoolVal())
	case *gnmi.TypedValue_BytesVal:
		return string(tv.GetBytesVal())
	case *gnmi.TypedValue_DecimalVal:
		//lint:ignore SA1019 still need DecimalVal for backward compatibility
		v := tv.GetDecimalVal()
		f := float64(v.Digits) / math.Pow10(int(v.Precision))
		return strconv.FormatFloat(f, 'e', -1, 64)
	case *gnmi.TypedValue_FloatVal:
		//lint:ignore SA1019 still need GetFloatVal for backward compatibility
		return strconv.FormatFloat(float64(tv.GetFloatVal()), 'e', -1, 64)
	case *gnmi.TypedValue_DoubleVal:
		return strconv.FormatFloat(tv.GetDoubleVal(), 'e', -1, 64)
	case *gnmi.TypedValue_IntVal:
		return strconv.Itoa(int(tv.GetIntVal()))
	case *gnmi.TypedValue_StringVal:
		return tv.GetStringVal()
	case *gnmi.TypedValue_UintVal:
		return strconv.Itoa(int(tv.GetUintVal()))
	case *gnmi.TypedValue_LeaflistVal:
		// TODO:
	case *gnmi.TypedValue_ProtoBytes:
		return string(tv.GetProtoBytes()) // ?
	case *gnmi.TypedValue_AnyVal:
		return string(tv.GetAnyVal().GetValue()) // ?
	case *gnmi.TypedValue_JsonIetfVal:
		jsondata := tv.GetJsonIetfVal()
		return string(jsondata)
	case *gnmi.TypedValue_JsonVal:
		jsondata := tv.GetJsonVal()
		return string(jsondata)
	}
	return ""
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Output:      os.Stderr,
		DisableTime: true,
	})

	logger.Info("starting plugin processor", "name", processorType)

	plug := &gNMIGetProcessor{
		m:    new(sync.RWMutex),
		vals: make(map[string]*readValues),
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "GNMIC_PLUGIN",
			MagicCookieValue: "gnmic",
		},
		Plugins: map[string]plugin.Plugin{
			processorType: &event_plugin.EventProcessorPlugin{Impl: plug},
		},
		Logger: logger,
	})
}
