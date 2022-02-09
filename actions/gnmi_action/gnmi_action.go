package gnmi_action

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"text/template"

	"github.com/karimra/gnmic/actions"
	"github.com/karimra/gnmic/api"
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
	defaultTarget   = `{{ index .Input.Tags "source" }}`
	defaultEncoding = "JSON"
	defaultFormat   = "json"
)

func init() {
	actions.Register(actionType, func() actions.Action {
		return &gnmiAction{
			logger:         log.New(io.Discard, "", 0),
			m:              new(sync.RWMutex),
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

	logger *log.Logger

	m              *sync.RWMutex
	targetsConfigs map[string]*types.TargetConfig
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

func (g *gnmiAction) Run(aCtx *actions.Context) (interface{}, error) {
	g.m.Lock()
	for n, tc := range aCtx.Targets {
		g.targetsConfigs[n] = tc
	}
	in := &actions.Context{
		Input:   aCtx.Input,
		Env:     aCtx.Env,
		Vars:    aCtx.Vars,
		Targets: aCtx.Targets,
	}
	g.m.Unlock()
	b := new(bytes.Buffer)
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
	resCh := make(chan *gnmiResponse)
	errCh := make(chan error)
	wg := new(sync.WaitGroup)
	wg.Add(len(targetsConfigs))
	for _, tc := range targetsConfigs {
		go func(tc *types.TargetConfig) {
			defer wg.Done()
			// create new actions.Context to be used by each target
			// run RPC
			rb, err := g.runRPC(ctx, tc, &actions.Context{
				Input: in.Input,
				Env:   in.Env,
				Vars:  in.Vars,
			})
			if err != nil {
				errCh <- err
				return
			}
			resCh <- &gnmiResponse{name: tc.Name, data: rb}
		}(tc)
	}

	errs := make([]error, 0)
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		for {
			select {
			case resp, ok := <-resCh:
				if !ok {
					return
				}
				var res interface{}
				// using yaml.Unmarshal instead of json.Unmarshal to avoid
				// treating integers as floats
				err = yaml.Unmarshal(resp.data, &res)
				if err != nil {
					errs = append(errs, err)
				}
				result[resp.name] = res
			case err := <-errCh:
				g.logger.Printf("gnmi action error: %v", err)
				errs = append(errs, err)
			case <-ctx.Done():
				return
			}
		}
	}()
	wg.Wait()
	close(resCh) // close result channel
	<-doneCh     // wait for the result map to be set
	if len(errs) > 0 {
		// return only the first errors
		return nil, errs[0]
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
	g.target, err = utils.CreateTemplate(fmt.Sprintf("%s-target", g.Name), g.Target)
	if err != nil {
		return err
	}
	g.prefix, err = utils.CreateTemplate(fmt.Sprintf("%s-prefix", g.Name), g.Prefix)
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
		tpl, err := utils.CreateTemplate(fmt.Sprintf("%s-%s-%d", g.Name, n, i), p)
		if err != nil {
			return nil, err
		}
		tpls = append(tpls, tpl)
	}
	return tpls, nil
}

func (g *gnmiAction) createGetRequest(in *actions.Context) (*gnmi.GetRequest, error) {
	gnmiOpts := make([]api.GNMIOption, 0, 3)
	gnmiOpts = append(gnmiOpts, api.Encoding(g.Encoding))
	gnmiOpts = append(gnmiOpts, api.DataType(g.Type))

	var err error
	b := new(bytes.Buffer)
	if g.Prefix != "" {
		err = g.prefix.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
		gnmiOpts = append(gnmiOpts, api.Prefix(b.String()))
	}

	for _, p := range g.paths {
		b.Reset()
		err = p.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		gnmiOpts = append(gnmiOpts, api.Path(b.String()))
	}

	return api.NewGetRequest(gnmiOpts...)
}

func (g *gnmiAction) createSetRequest(in *actions.Context) (*gnmi.SetRequest, error) {
	gnmiOpts := make([]api.GNMIOption, 0, len(g.paths))
	var err error
	b := new(bytes.Buffer)
	if g.Prefix != "" {
		err = g.prefix.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
		gnmiOpts = append(gnmiOpts, api.Prefix(b.String()))
	}
	for i, p := range g.paths {
		b.Reset()
		err = p.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		sPath := b.String()
		switch g.RPC {
		case "set-delete":
			gnmiOpts = append(gnmiOpts, api.Delete(sPath))
		case "set-update":
			b.Reset()
			err = g.values[i].Execute(b, in)
			if err != nil {
				return nil, fmt.Errorf("value %d parse error: %v", i, err)
			}
			gnmiOpts = append(gnmiOpts, api.Update(
				api.Path(sPath),
				api.Value(b.String(), g.Encoding),
			))
		case "set-replace":
			b.Reset()
			err = g.values[i].Execute(b, in)
			if err != nil {
				return nil, fmt.Errorf("value %d parse error: %v", i, err)
			}
			gnmiOpts = append(gnmiOpts,
				api.Replace(
					api.Path(sPath),
					api.Value(b.String(), g.Encoding),
				))
		}
	}
	return api.NewSetRequest(gnmiOpts...)
}

func (g *gnmiAction) createSubscribeRequest(in *actions.Context) (*gnmi.SubscribeRequest, error) {
	gnmiOpts := make([]api.GNMIOption, 0, 2+len(g.paths))
	gnmiOpts = append(gnmiOpts,
		api.Encoding(g.Encoding),
		api.SubscriptionListModeONCE(),
	)
	//
	var err error
	b := new(bytes.Buffer)
	if g.Prefix != "" {
		err = g.prefix.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("prefix template exec error: %v", err)
		}
		gnmiOpts = append(gnmiOpts, api.Prefix(b.String()))
	}
	for _, p := range g.paths {
		b.Reset()
		err = p.Execute(b, in)
		if err != nil {
			return nil, fmt.Errorf("path template exec error: %v", err)
		}
		gnmiOpts = append(gnmiOpts, api.Subscription(
			api.Path(b.String())))
	}
	return api.NewSubscribeRequest(gnmiOpts...)
}

func (g *gnmiAction) selectTargets(tName string) ([]*types.TargetConfig, error) {
	if tName == "" {
		return nil, nil
	}

	targets := make([]*types.TargetConfig, 0, len(g.targetsConfigs))
	g.m.RLock()
	defer g.m.RUnlock()
	// select all targets
	if tName == "all" {
		for _, tc := range g.targetsConfigs {
			targets = append(targets, tc)
		}
		return targets, nil
	}
	// select a few targets
	tNames := strings.Split(tName, ",")
	for _, name := range tNames {
		if tc, ok := g.targetsConfigs[name]; ok {
			targets = append(targets, tc)
		}
	}
	return targets, nil
}

func (g *gnmiAction) runRPC(ctx context.Context, tc *types.TargetConfig, in *actions.Context) ([]byte, error) {
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

func (g *gnmiAction) runGet(ctx context.Context, tc *types.TargetConfig, in *actions.Context) ([]byte, error) {
	t := target.NewTarget(tc)
	req, err := g.createGetRequest(in)
	if err != nil {
		return nil, err
	}
	err = t.CreateGNMIClient(ctx)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	resp, err := t.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("target %q GetRequest failed: %v", t.Config.Name, err)
	}
	mo := &formatters.MarshalOptions{Format: g.Format}
	return mo.Marshal(resp, nil)
}

func (g *gnmiAction) runSet(ctx context.Context, tc *types.TargetConfig, in *actions.Context) ([]byte, error) {
	t := target.NewTarget(tc)
	req, err := g.createSetRequest(in)
	if err != nil {
		return nil, err
	}
	err = t.CreateGNMIClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("target %q SetRequest failed: %v", t.Config.Name, err)
	}
	defer t.Close()
	resp, err := t.Set(ctx, req)
	if err != nil {
		return nil, err
	}
	mo := &formatters.MarshalOptions{Format: g.Format}
	return mo.Marshal(resp, nil)
}

func (g *gnmiAction) runSubscribe(ctx context.Context, tc *types.TargetConfig, in *actions.Context) ([]byte, error) {
	t := target.NewTarget(tc)
	req, err := g.createSubscribeRequest(in)
	if err != nil {
		return nil, err
	}
	err = t.CreateGNMIClient(ctx)
	if err != nil {
		return nil, err
	}
	defer t.Close()
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

type gnmiResponse struct {
	name string
	data []byte
}
