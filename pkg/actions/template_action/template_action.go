// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package template_action

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"text/template"

	"github.com/openconfig/gnmic/pkg/actions"
	"github.com/openconfig/gnmic/pkg/gtemplate"
)

const (
	loggingPrefix   = "[template_action] "
	actionType      = "template"
	defaultTemplate = "{{ . }}"
)

func init() {
	actions.Register(actionType, func() actions.Action {
		return &templateAction{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

type templateAction struct {
	Name         string `mapstructure:"name,omitempty"`
	Template     string `mapstructure:"template,omitempty"`
	TemplateFile string `mapstructure:"template-file,omitempty"`
	Output       string `mapstructure:"output,omitempty"`
	Debug        bool   `mapstructure:"debug,omitempty"`

	tpl    *template.Template
	logger *log.Logger
}

func (t *templateAction) Init(cfg map[string]interface{}, opts ...actions.Option) error {
	err := actions.DecodeConfig(cfg, t)
	if err != nil {
		return err
	}

	for _, opt := range opts {
		opt(t)
	}
	if t.Name == "" {
		return fmt.Errorf("action type %q missing name field", actionType)
	}
	err = t.setDefaults()
	if err != nil {
		return err
	}
	if t.Template != "" {
		t.tpl, err = gtemplate.CreateTemplate(fmt.Sprintf("%s-template-action", t.Name), t.Template)
		if err != nil {
			return err
		}
	} else if t.TemplateFile != "" {
		t.tpl, err = template.ParseGlob(t.TemplateFile)
		if err != nil {
			return err
		}
		t.tpl = t.tpl.Funcs(gtemplate.NewTemplateEngine().CreateFuncs()).
			Option("missingkey=zero")
	}
	t.logger.Printf("action name %q of type %q initialized: %v", t.Name, actionType, t)
	return nil
}

func (t *templateAction) Run(_ context.Context, aCtx *actions.Context) (interface{}, error) {
	b := new(bytes.Buffer)
	err := t.tpl.Execute(b, &actions.Context{
		Input:   aCtx.Input,
		Env:     aCtx.Env,
		Vars:    aCtx.Vars,
		Targets: aCtx.Targets,
	})
	if err != nil {
		return nil, err
	}
	out := b.String()
	if t.Debug {
		t.logger.Printf("template output: %s", out)
	}
	switch t.Output {
	case "stdout":
		fmt.Fprint(os.Stdout, out)
	case "":
	default:
		fi, err := os.Create(t.Output)
		if err != nil {
			return nil, err
		}
		_, err = fi.Write(b.Bytes())
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (t *templateAction) NName() string { return t.Name }

func (t *templateAction) setDefaults() error {
	if t.Template == "" && t.TemplateFile == "" {
		t.Template = defaultTemplate
	}
	return nil
}
