package gnmi_action

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	"github.com/karimra/gnmic/actions"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/target"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/proto/gnmi"
	"gopkg.in/yaml.v2"
)

const (
	defaultRPC      = "get"
	loggingPrefix   = "[gnmi_action] "
	actionType      = "gnmi"
	defaultDataType = "ALL"
	defaultTarget   = `{{ index .Event.Tags "source" }}`
	defaultEncoding = "JSON"
	defaultFormat   = "json"
)

func init() {
	actions.Register(actionType, func() actions.Action {
		return &gnmiAction{
			logger:         log.New(io.Discard, "", 0),
			targetsConfigs: make(map[string]*types.TargetConfig),
		}
	})
}

type gnmiAction struct {
	// action name
	Name string `mapstructure:"name,omitempty"`
	// target of the gNMI RPC, it can be a Go template
	Target string `mapstructure:"target,omitempty"`
	// gNMI RPC, possible values `get`, `set`, `set-update`,
	// `set-replace`, `sub`, `subscribe`
	RPC string `mapstructure:"rpc,omitempty"`
	// gNMI Path Prefix, can be a Go template
	Prefix string `mapstructure:"prefix,omitempty"`
	// list of gNMI Paths, each one can be a Go template
	Paths []string `mapstructure:"paths,omitempty"`
	// gNMI data type in case RPC is `get`,
	// possible values: `config`, `state`, `operational`
	Type string `mapstructure:"data-type,omitempty"`
	// list of gNMI values, used in case RPC=`set*`
	Values []string `mapstructure:"values,omitempty"`
	// gNMI encoding
	Encoding string `mapstructure:"encoding,omitempty"`
	// Debug
	Debug bool `mapstructure:"debug,omitempty"`
	// Ignore ENV proxy
	NoEnvProxy bool `mapstructure:"no-env-proxy,omitempty"`
	// Response format,
	// possible values: `json`, `event`, `prototext`, `protojson`
	Format string `mapstructure:"format,omitempty"`

	target *template.Template
	prefix *template.Template
	paths  []*template.Template
	values []*template.Template

	targetsConfigs map[string]*types.TargetConfig
	logger         *log.Logger
}

func (g *gnmiAction) Init(cfg map[string]interface{}, opts ...actions.Option) error {
	err := actions.DecodeConfig(cfg, g)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(g)
	}
	if g.Name == "" {
		return fmt.Errorf("action type %q missing name field", actionType)
	}
	g.setDefaults()
	err = g.parseTemplates()
	if err != nil {
		return err
	}
	err = g.validate()
	if err != nil {
		return err
	}
	g.logger.Printf("action name %q of type %q initialized: %v", g.Name, actionType, g)
	return nil
}

func (g *gnmiAction) Run(e *formatters.EventMsg, env, vars map[string]interface{}) (interface{}, error) {
	b := new(bytes.Buffer)
	in := &actions.Input{
		Event: e,
		Env:   env,
		Vars:  vars,
	}
	err := g.target.Execute(b, in)
	if err != nil {
		return nil, err
	}
	tName := b.String()
	targetsConfigs, err := g.selectTargets(tName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(map[string]interface{})
	for _, tc := range targetsConfigs {
		rb, err := g.runRPC(ctx, tc, &actions.Input{
			Event:  in.Event,
			Env:    in.Env,
			Vars:   in.Vars,
			Target: tc.Name,
		})
		if err != nil {
			return nil, err
		}
		var res interface{}
		// using yaml.Unmarshal instead of json.Unmarshal to avoid
		// treating integers as floats
		err = yaml.Unmarshal(rb, &res)
		if err != nil {
			return nil, err
		}
		result[tc.Name] = res
	}
	return result, nil
}

func (g *gnmiAction) NName() string { return g.Name }

func (g *gnmiAction) setDefaults() {
	if g.Type == "" {
		g.Type = defaultDataType
	}
	if g.Encoding == "" {
		g.Encoding = defaultEncoding
	}
	if g.RPC == "" {
		g.RPC = defaultRPC
	}
	if g.RPC == "set" {
		g.RPC = "set-update"
	}
	if g.Target == "" {
		g.Target = defaultTarget
	}
	if g.Format == "" {
		g.Format = defaultFormat
	}
}

func (g *gnmiAction) validate() error {
	numPaths := len(g.Paths)
	if numPaths == 0 {
		return errors.New("paths field is required")
	}
	switch g.RPC {
	case "get", "delete":
	case "set-update", "set-replace":
		numValues := len(g.values)
		if numValues == 0 {
			return errors.New("values field is required when RPC is set")
		}
		if numPaths != len(g.values) {
			return errors.New("number of paths and values do not match")
		}
	case "sub", "subscribe":
		if strings.ToLower(g.Format) != "json" &&
			strings.ToLower(g.Format) != "protojson" &&
			strings.ToLower(g.Format) != "event" {
			return fmt.Errorf("unsupported format %q", g.Format)
		}
	default:
		return fmt.Errorf("unknown gnmi RPC %q", g.RPC)
	}
	return nil
}

func (g *gnmiAction) parseTemplates() error {
	var err error
	g.target, err = template.New("target").
		Funcs(gomplate.CreateFuncs(context.TODO(), new(data.Data))).
		Parse(g.Target)
	if err != nil {
		return err
	}
	g.prefix, err = template.New("prefix").
		Funcs(gomplate.CreateFuncs(context.TODO(), new(data.Data))).
		Parse(g.Prefix)
	if err != nil {
		return err
	}
	g.paths, err = g.createTemplates("path", g.Paths)
	if err != nil {
		return err
	}
	g.values, err = g.createTemplates("value", g.Values)
	return err
}

func (g *gnmiAction) createTemplates(n string, s []string) ([]*template.Template, error) {
	tpls := make([]*template.Template, 0, len(s))
	for i, p := range s {
		tpl, err := template.New(fmt.Sprintf("%s-%d", n, i)).
			Funcs(gomplate.CreateFuncs(context.TODO(), new(data.Data))).
			Parse(p)
		if err != nil {
			return nil, err
		}
		tpls = append(tpls, tpl)
	}
	return tpls, nil
}

func (g *gnmiAction) createGetRequest(in *actions.Input) (*gnmi.GetRequest, error) {
	encodingVal, ok := gnmi.Encoding_value[strings.Replace(strings.ToUpper(g.Encoding), "-", "_", -1)]
	if !ok {
		return nil, fmt.Errorf("invalid encoding type '%s'", g.Encoding)
	}
	req := &gnmi.GetRequest{
		UseModels: make([]*gnmi.ModelData, 0),
		Path:      make([]*gnmi.Path, 0, len(g.paths)),
		Encoding:  gnmi.Encoding(encodingVal),
	}
	var err error
	b := new(bytes.Buffer)
	if g.Prefix != "" {
		err = g.prefix.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}

		gnmiPrefix, err := utils.ParsePath(b.String())
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
		req.Prefix = gnmiPrefix
	}
	if g.Type != "" {
		dti, ok := gnmi.GetRequest_DataType_value[strings.ToUpper(g.Type)]
		if !ok {
			return nil, fmt.Errorf("unknown data type %s", g.Type)
		}
		req.Type = gnmi.GetRequest_DataType(dti)
	}
	for _, p := range g.paths {
		b.Reset()
		err = p.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(b.String()))
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		req.Path = append(req.Path, gnmiPath)
	}
	return req, nil
}

func (g *gnmiAction) createSetRequest(in *actions.Input) (*gnmi.SetRequest, error) {
	req := &gnmi.SetRequest{
		Delete:  make([]*gnmi.Path, 0, len(g.paths)),
		Replace: make([]*gnmi.Update, 0, len(g.paths)),
		Update:  make([]*gnmi.Update, 0, len(g.paths)),
	}
	var err error
	b := new(bytes.Buffer)
	if g.Prefix != "" {
		err = g.prefix.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
		gnmiPrefix, err := utils.ParsePath(b.String())
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
		req.Prefix = gnmiPrefix
	}
	for i, p := range g.paths {
		b.Reset()
		err = p.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(b.String()))
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}

		// value
		b.Reset()
		err = g.values[i].Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("value %d parse error: %v", i, err)
		}
		val, err := g.createTypedValue(b.String())
		if err != nil {
			return nil, fmt.Errorf("create value %d error: %v", i, err)
		}
		switch g.RPC {
		case "set-update":
			req.Update = append(req.Update, &gnmi.Update{
				Path: gnmiPath,
				Val:  val,
			})
		case "set-replace":
			req.Replace = append(req.Replace, &gnmi.Update{
				Path: gnmiPath,
				Val:  val,
			})
		}
	}
	return req, nil
}

func (g *gnmiAction) createTypedValue(val string) (*gnmi.TypedValue, error) {
	var err error
	value := new(gnmi.TypedValue)
	switch strings.ToLower(g.Encoding) {
	case "json":
		buff := new(bytes.Buffer)
		val := strings.TrimRight(strings.TrimLeft(val, "["), "]")
		bval := json.RawMessage(val)
		if json.Valid(bval) {
			err = json.NewEncoder(buff).Encode(bval)
		} else {
			err = json.NewEncoder(buff).Encode(val)
		}
		if err != nil {
			return nil, err
		}
		value.Value = &gnmi.TypedValue_JsonVal{
			JsonVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
		}
	case "json_ietf":
		buff := new(bytes.Buffer)
		val := strings.TrimRight(strings.TrimLeft(val, "["), "]")
		bval := json.RawMessage(val)
		if json.Valid(bval) {
			err = json.NewEncoder(buff).Encode(bval)
		} else {
			err = json.NewEncoder(buff).Encode(val)
		}
		if err != nil {
			return nil, err
		}
		value.Value = &gnmi.TypedValue_JsonIetfVal{
			JsonIetfVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
		}
	case "ascii":
		value.Value = &gnmi.TypedValue_AsciiVal{
			AsciiVal: val,
		}
	case "bool":
		bval, err := strconv.ParseBool(val)
		if err != nil {
			return nil, err
		}
		value.Value = &gnmi.TypedValue_BoolVal{
			BoolVal: bval,
		}
	case "bytes":
		value.Value = &gnmi.TypedValue_BytesVal{
			BytesVal: []byte(val),
		}
	case "decimal":
		dVal := &gnmi.Decimal64{}
		value.Value = &gnmi.TypedValue_DecimalVal{
			DecimalVal: dVal,
		}
		return nil, fmt.Errorf("decimal type not implemented")
	case "float":
		f, err := strconv.ParseFloat(val, 32)
		if err != nil {
			return nil, err
		}
		value.Value = &gnmi.TypedValue_FloatVal{
			FloatVal: float32(f),
		}
	case "int":
		k, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, err
		}
		value.Value = &gnmi.TypedValue_IntVal{
			IntVal: k,
		}
	case "uint":
		u, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return nil, err
		}
		value.Value = &gnmi.TypedValue_UintVal{
			UintVal: u,
		}
	case "string":
		value.Value = &gnmi.TypedValue_StringVal{
			StringVal: val,
		}
	default:
		return nil, fmt.Errorf("unknown type %q", g.Encoding)
	}
	return value, nil
}

func (g *gnmiAction) createSubscribeRequest(in *actions.Input) (*gnmi.SubscribeRequest, error) {
	encodingVal, ok := gnmi.Encoding_value[strings.Replace(strings.ToUpper(g.Encoding), "-", "_", -1)]
	if !ok {
		return nil, fmt.Errorf("invalid encoding type '%s'", g.Encoding)
	}

	var err error
	b := new(bytes.Buffer)
	var gnmiPrefix *gnmi.Path
	if g.Prefix != "" {
		err = g.prefix.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}

		gnmiPrefix, err = utils.ParsePath(b.String())
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
	}
	var subscriptions = make([]*gnmi.Subscription, 0, len(g.paths))
	for _, p := range g.paths {
		b.Reset()
		err = p.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(b.String()))
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		subscriptions = append(subscriptions, &gnmi.Subscription{
			Path: gnmiPath,
		})
	}

	return &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{
				Prefix:       gnmiPrefix,
				Encoding:     gnmi.Encoding(encodingVal),
				Mode:         gnmi.SubscriptionList_ONCE,
				Subscription: subscriptions,
			},
		},
	}, nil
}

func (g *gnmiAction) selectTargets(tName string) ([]*types.TargetConfig, error) {
	if tName == "" {
		return nil, nil
	}
	targets := make([]*types.TargetConfig, 0, len(g.targetsConfigs))
	if tName == "all" {
		for _, tc := range g.targetsConfigs {
			targets = append(targets, tc)
		}
		return targets, nil
	}
	tNames := strings.Split(tName, ",")
	for _, name := range tNames {
		if tc, ok := g.targetsConfigs[name]; ok {
			targets = append(targets, tc)
		}
	}
	return targets, nil
}

func (g *gnmiAction) runRPC(ctx context.Context, tc *types.TargetConfig, in *actions.Input) ([]byte, error) {
	switch g.RPC {
	case "get":
		return g.runGet(ctx, tc, in)
	case "set-update", "set-replace", "delete":
		return g.runSet(ctx, tc, in)
	case "sub", "subscribe": // once
		return g.runSubscribe(ctx, tc, in)
	default:
		return nil, fmt.Errorf("unknown RPC %q", g.RPC)
	}
}

func (g *gnmiAction) runGet(ctx context.Context, tc *types.TargetConfig, in *actions.Input) ([]byte, error) {
	t := &target.Target{Config: tc}
	req, err := g.createGetRequest(in)
	if err != nil {
		return nil, err
	}
	err = t.CreateGNMIClient(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := t.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	mo := &formatters.MarshalOptions{Format: g.Format}
	return mo.Marshal(resp, nil)
}

func (g *gnmiAction) runSet(ctx context.Context, tc *types.TargetConfig, in *actions.Input) ([]byte, error) {
	t := &target.Target{Config: tc}
	req, err := g.createSetRequest(in)
	if err != nil {
		return nil, err
	}
	err = t.CreateGNMIClient(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := t.Set(ctx, req)
	if err != nil {
		return nil, err
	}
	mo := &formatters.MarshalOptions{Format: g.Format}
	return mo.Marshal(resp, nil)
}

func (g *gnmiAction) runSubscribe(ctx context.Context, tc *types.TargetConfig, in *actions.Input) ([]byte, error) {
	t := &target.Target{Config: tc}
	req, err := g.createSubscribeRequest(in)
	if err != nil {
		return nil, err
	}
	err = t.CreateGNMIClient(ctx)
	if err != nil {
		return nil, err
	}

	responses, err := t.SubscribeOnce(ctx, req)
	if err != nil {
		return nil, err
	}
	mo := &formatters.MarshalOptions{Format: g.Format}
	formattedResponse := make([]interface{}, 0, len(responses))
	m := map[string]string{
		"source": tc.Name,
	}
	for _, r := range responses {
		msgb, err := mo.Marshal(r, m)
		if err != nil {
			return nil, err
		}
		var v interface{}
		err = json.Unmarshal(msgb, &v)
		if err != nil {
			return nil, err
		}
		formattedResponse = append(formattedResponse, utils.Convert(v))
	}
	return json.Marshal(formattedResponse)
}
