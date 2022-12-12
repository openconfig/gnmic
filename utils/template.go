// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"path"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
)

func CreateTemplate(name, text string) (*template.Template, error) {
	return template.New(name).
		Option("missingkey=zero").
		Funcs(gomplate.CreateFuncs(context.TODO(), new(data.Data))).
		Parse(text)
}

func CreateFileTemplate(filename string) (*template.Template, error) {
	name := path.Base(filename)

	tpl, err := template.New(name).
		Funcs(gomplate.CreateFuncs(context.TODO(), new(data.Data))).
		ParseFiles(filename)

	template.Must(tpl, err)

	return tpl, err
}
