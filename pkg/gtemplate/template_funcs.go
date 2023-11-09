// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package gtemplate

import (
	"context"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
)

type templateEngine interface {
	CreateFuncs() template.FuncMap
}

func NewTemplateEngine() templateEngine {
	return &gmplt{}
}

type gmplt struct{}

func (*gmplt) CreateFuncs() template.FuncMap {
	return gomplate.CreateFuncs(context.TODO(), new(data.Data))
}
